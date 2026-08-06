package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"voice-rooms/internal/abuse"
	"voice-rooms/internal/antispam"
	"voice-rooms/internal/app"
	appgroups "voice-rooms/internal/groups"
	"voice-rooms/internal/groups/hub"
	"voice-rooms/internal/httpapi"
	"voice-rooms/internal/store"
)

func banAPI(t *testing.T) *testAPI {
	t.Helper()
	return banAPIWith(t, nil)
}

func banAPIWith(t *testing.T, mutate func(*abuse.Config)) *testAPI {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "bans.db"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	voice := newVoiceFake()
	service := app.New(st, voice, time.Hour, 15*time.Minute, 4, logger)
	cfg := abuse.DefaultConfig()
	cfg.RegisterLimit = abuse.Limit{Capacity: 2, Refill: time.Hour}
	cfg.TrustedProxies = []string{"127.0.0.1"}
	cfg.IPBanThreshold = 3
	if mutate != nil {
		mutate(&cfg)
	}
	limiter := abuse.NewLimiter(st, cfg)
	groupService := appgroups.New(st, voice, hub.New()).
		WithMessageGuard(antispam.NewGuard(limiter, st, cfg))
	server := httptest.NewServer(httpapi.NewWithSecurity(service, groupService, st, logger, []string{"https://client.test"}, cfg))
	t.Cleanup(func() {
		server.Close()
		st.Close()
	})
	return &testAPI{server: server, store: st, voice: voice}
}

func TestIPBanAfterRepeatedRateLimit(t *testing.T) {
	api := banAPI(t)
	for i := 0; i < 5; i++ {
		status, _, _ := api.request(t, http.MethodPost, "/v1/auth/register", "", []byte(
			fmt.Sprintf(`{"email":"spam%d@example.com","password":"password123"}`, i),
		))
		want := http.StatusCreated
		if i >= 2 {
			want = http.StatusTooManyRequests
		}
		if status != want {
			t.Fatalf("register %d: got %d, want %d", i, status, want)
		}
	}
	status, body, headers := api.request(t, http.MethodPost, "/v1/auth/register", "", []byte(
		`{"email":"blocked@example.com","password":"password123"}`,
	))
	if status != http.StatusTooManyRequests || !bytes.Contains(body, []byte(`"ip_banned"`)) || headers.Get("Retry-After") == "" {
		t.Fatalf("IP was not banned: %d %s", status, body)
	}
}

// TestBanEscalationDeletesRepeatedOffender drives the IP ban ladder to the
// maximum level from an authenticated request and verifies that two maximum-
// level bans within the window delete the offending account.
func TestBanEscalationDeletesRepeatedOffender(t *testing.T) {
	api := banAPIWith(t, func(cfg *abuse.Config) {
		cfg.RegisterLimit = abuse.Limit{Capacity: 1, Refill: time.Hour}
		cfg.IPBanThreshold = 1
		cfg.IPBanSteps = []time.Duration{time.Second, time.Second, time.Second, time.Second}
		cfg.AccountBanDeletionCount = 2
	})
	token := api.user(t, "offender")
	spam := []byte(`{"email":"spam@example.com","password":"password123"}`)
	trigger := func() {
		status, body, _ := api.request(t, http.MethodPost, "/v1/auth/register", token, spam)
		if status != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d %s", status, body)
		}
	}
	for i := 0; i < 5; i++ {
		trigger()
		time.Sleep(1200 * time.Millisecond)
	}
	status, body, _ := api.request(t, http.MethodGet, "/v1/me", token, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("offender account must be deleted after repeated maximum-level bans: %d %s", status, body)
	}
}

func TestBanCleanupRemovesRecentMessagesAndDisconnectsVoice(t *testing.T) {
	api := banAPIWith(t, func(cfg *abuse.Config) {
		cfg.RegisterLimit = abuse.Limit{Capacity: 1, Refill: time.Hour}
		cfg.IPBanThreshold = 1
		cfg.IPBanSteps = []time.Duration{time.Second}
		cfg.MessageLimit = abuse.Limit{Capacity: 200, Refill: time.Hour}
		cfg.NewAccountMessageCap = 200
		cfg.MinMessageInterval = 0
	})
	token := api.user(t, "cleanup-offender")
	status, body, _ := api.request(t, http.MethodGet, "/v1/me", token, nil)
	if status != http.StatusOK {
		t.Fatalf("me before ban: %d %s", status, body)
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &me); err != nil {
		t.Fatal(err)
	}
	groupID := createGroup(t, api, token, "Cleanup Group")
	for i := 0; i < 55; i++ {
		status, body, _ = api.request(t, http.MethodPost, "/v1/groups/"+groupID+"/messages", token, []byte(fmt.Sprintf(`{"body":"msg %02d"}`, i)))
		if status != http.StatusCreated {
			t.Fatalf("create message %d: %d %s", i, status, body)
		}
	}
	status, body, _ = api.request(t, http.MethodPost, "/v1/groups/"+groupID+"/voice-rooms", token, []byte(`{"name":"General"}`))
	if status != http.StatusCreated {
		t.Fatalf("create voice room: %d %s", status, body)
	}
	var room struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &room); err != nil {
		t.Fatal(err)
	}
	status, body, _ = api.request(t, http.MethodPost, "/v1/group-voice-rooms/"+room.ID+"/join", token, nil)
	if status != http.StatusOK {
		t.Fatalf("join voice room: %d %s", status, body)
	}
	status, body, _ = api.request(t, http.MethodPost, "/v1/auth/register", token, []byte(`{"email":"spam@example.com","password":"password123"}`))
	if status != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d %s", status, body)
	}
	req, err := http.NewRequest(http.MethodGet, api.server.URL+"/v1/groups/"+groupID+"/messages?limit=100", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", "198.51.100.10")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("messages after ban: %d %s", resp.StatusCode, body)
	}
	var messages []map[string]any
	if err := json.Unmarshal(body, &messages); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected 0 remaining messages after cleanup, got %d", len(messages))
	}
	removed := api.voice.removedParticipants()
	if len(removed) != 1 || removed[0] != "group:"+groupID+":"+room.ID+":"+me.ID {
		t.Fatalf("unexpected removed participants: %#v", removed)
	}
}

func TestRepeatedMessagesAreDeletedByAntispam(t *testing.T) {
	api := banAPIWith(t, func(cfg *abuse.Config) {
		cfg.MessageLimit = abuse.Limit{Capacity: 200, Refill: time.Hour}
		cfg.NewAccountMessageCap = 200
		cfg.MinMessageInterval = 0
	})
	token := api.user(t, "repeat-offender")
	groupID := createGroup(t, api, token, "Repeat Cleanup")
	status, body, _ := api.request(t, http.MethodPost, "/v1/groups/"+groupID+"/messages", token, []byte(`{"body":"одно и то же"}`))
	if status != http.StatusCreated {
		t.Fatalf("create first message: %d %s", status, body)
	}
	status, body, _ = api.request(t, http.MethodPost, "/v1/groups/"+groupID+"/messages", token, []byte(`{"body":"одно и то же"}`))
	if status != http.StatusConflict || !bytes.Contains(body, []byte(`"spam_warning"`)) {
		t.Fatalf("repeat warning: %d %s", status, body)
	}
	req, err := http.NewRequest(http.MethodGet, api.server.URL+"/v1/groups/"+groupID+"/messages?limit=20", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", "198.51.100.11")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("messages after repeat cleanup: %d %s", resp.StatusCode, body)
	}
	var messages []map[string]any
	if err := json.Unmarshal(body, &messages); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected repeated message to be deleted, got %d messages", len(messages))
	}
}

func TestSpamWarningsAccumulateAndDeleteGroup(t *testing.T) {
	api := banAPIWith(t, func(cfg *abuse.Config) {
		cfg.RegisterLimit = abuse.Limit{Capacity: 10, Refill: time.Hour}
		cfg.MessageLimit = abuse.Limit{Capacity: 200, Refill: time.Second}
		cfg.NewAccountMessageCap, cfg.MinMessageInterval = 200, 0
	})
	owner, a, b, c := api.user(t, "warn-owner"), api.user(t, "warn-a"), api.user(t, "warn-b"), api.user(t, "warn-c")
	groupID := createGroup(t, api, owner, "Spam Watch")
	for _, token := range []string{a, b, c} {
		if status, body, _ := api.request(t, http.MethodPost, "/v1/groups/"+groupID+"/members", token, nil); status != http.StatusOK {
			t.Fatalf("join group: %d %s", status, body)
		}
	}
	for i, text := range []string{"залетай в канал", "новый канал для всех", "скидки только сегодня"} {
		if status, body, _ := api.request(t, http.MethodPost, "/v1/groups/"+groupID+"/messages", a, []byte(fmt.Sprintf(`{"body":"%s 1"}`, text))); status != http.StatusCreated {
			t.Fatalf("message a %d: %d %s", i, status, body)
		}
		if status, body, _ := api.request(t, http.MethodPost, "/v1/groups/"+groupID+"/messages", b, []byte(fmt.Sprintf(`{"body":"%s 2"}`, text))); status != http.StatusCreated {
			t.Fatalf("message b %d: %d %s", i, status, body)
		}
		status, body, _ := api.request(t, http.MethodPost, "/v1/groups/"+groupID+"/messages", c, []byte(fmt.Sprintf(`{"body":"%s 3"}`, text)))
		if status != http.StatusConflict || !bytes.Contains(body, []byte(`"spam_warning"`)) {
			t.Fatalf("message c %d: %d %s", i, status, body)
		}
		if i == 1 {
			status, body, _ = api.request(t, http.MethodGet, "/v1/groups/"+groupID+"/profile", owner, nil)
			if status != http.StatusOK || bytes.Count(body, []byte(`"reason"`)) != 2 {
				t.Fatalf("profile warnings: %d %s", status, body)
			}
		}
		time.Sleep(1100 * time.Millisecond)
	}
	if status, _, _ := api.request(t, http.MethodGet, "/v1/groups/"+groupID, owner, nil); status != http.StatusNotFound {
		t.Fatalf("group should be auto deleted after third warning: %d", status)
	}
}

func TestUserOwnershipGroupLimit(t *testing.T) {
	api := newAPI(t, 4)
	owner := api.user(t, "limit-owner")
	for i := 0; i < 3; i++ {
		status, body, _ := api.request(t, http.MethodPost, "/v1/groups", owner, []byte(
			fmt.Sprintf(`{"name":"Group %d","visibility":"public"}`, i),
		))
		if status != http.StatusCreated {
			t.Fatalf("create group %d: %d %s", i, status, body)
		}
	}
	status, body, _ := api.request(t, http.MethodPost, "/v1/groups", owner, []byte(
		`{"name":"Fourth","visibility":"public"}`,
	))
	if status != http.StatusConflict || !bytes.Contains(body, []byte(`"group_limit"`)) {
		t.Fatalf("fourth group was not limited: %d %s", status, body)
	}
}
