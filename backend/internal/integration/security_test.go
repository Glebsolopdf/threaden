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
	"strconv"
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

func strictAPI(t *testing.T) *testAPI {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "security.db"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	voice := newVoiceFake()
	service := app.New(st, voice, time.Hour, 15*time.Minute, 4, logger)
	cfg := abuse.DefaultConfig()
	cfg.RegisterLimit = abuse.Limit{Capacity: 1, Refill: time.Hour}
	cfg.LoginLimit = abuse.Limit{Capacity: 1, Refill: time.Hour}
	cfg.TrustedProxies = []string{"127.0.0.1"}
	limiter := abuse.NewLimiter(st, cfg)
	groupService := appgroups.New(st, voice, hub.New()).
		WithMessageGuard(antispam.NewGuard(limiter, st, cfg))
	server := httptest.NewServer(httpapi.NewWithSecurity(service, groupService, st, logger, []string{"https://client.test"}, cfg))
	t.Cleanup(func() {
		server.Close()
		st.Close()
	})
	return &testAPI{server: server, store: st}
}

func TestRegisterRateLimitReturnsRetryAfter(t *testing.T) {
	api := strictAPI(t)
	for i := range 2 {
		status, body, headers := api.request(t, http.MethodPost, "/v1/auth/register", "", []byte(
			fmt.Sprintf(`{"email":"limited%d@example.com","password":"password123"}`, i),
		))
		if i == 0 && status != http.StatusCreated {
			t.Fatalf("first register: %d %s", status, body)
		}
		if i == 1 {
			if status != http.StatusTooManyRequests || headers.Get("Retry-After") == "" {
				t.Fatalf("rate limit response: %d retry=%q body=%s", status, headers.Get("Retry-After"), body)
			}
		}
	}
}

func TestProfileAndGroupCreationUseThreeMinuteLimit(t *testing.T) {
	api := strictAPI(t)
	owner := api.user(t, "profile-group-limit")

	status, body, _ := api.request(t, http.MethodPost, "/v1/groups", owner, []byte(`{"name":"First","visibility":"public"}`))
	if status != http.StatusCreated {
		t.Fatalf("first group: %d %s", status, body)
	}
	status, body, headers := api.request(t, http.MethodPost, "/v1/groups", owner, []byte(`{"name":"Second","visibility":"public"}`))
	if status != http.StatusTooManyRequests || retryAfterSeconds(t, headers) < 170 {
		t.Fatalf("second group was not limited: %d retry=%q %s", status, headers.Get("Retry-After"), body)
	}

	status, body, _ = updateProfile(t, api, owner, "Changed", []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"))
	if status != http.StatusOK {
		t.Fatalf("first profile update: %d %s", status, body)
	}
	status, body, headers = updateProfile(t, api, owner, "Changed again", []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"))
	if status != http.StatusTooManyRequests || retryAfterSeconds(t, headers) < 170 {
		t.Fatalf("second profile update was not limited: %d retry=%q %s", status, headers.Get("Retry-After"), body)
	}
}

func retryAfterSeconds(t *testing.T, headers http.Header) int {
	t.Helper()
	seconds, err := strconv.Atoi(headers.Get("Retry-After"))
	if err != nil {
		t.Fatalf("invalid Retry-After header: %q", headers.Get("Retry-After"))
	}
	return seconds
}

func sessionTokenFromHeaders(t *testing.T, headers http.Header) string {
	t.Helper()
	for _, cookie := range (&http.Response{Header: headers}).Cookies() {
		if cookie.Name != "threaden_session" {
			continue
		}
		if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Value == "" {
			t.Fatalf("insecure session cookie: %+v", cookie)
		}
		return cookie.Value
	}
	t.Fatal("session cookie missing")
	return ""
}

func TestUnchangedProfileIsRejected(t *testing.T) {
	api := newAPI(t, 2)
	owner := api.user(t, "unchanged-profile")
	avatar := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")
	if status, body, _ := updateProfile(t, api, owner, "Changed", avatar); status != http.StatusOK {
		t.Fatalf("first profile update: %d %s", status, body)
	}
	status, body, _ := updateProfile(t, api, owner, "Changed", avatar)
	if status != http.StatusConflict || !bytes.Contains(body, []byte(`"profile_unchanged"`)) {
		t.Fatalf("unchanged profile accepted: %d %s", status, body)
	}
}

func TestAnonymousLoginLimitsAreIsolatedByIP(t *testing.T) {
	api := strictAPI(t)
	for _, ip := range []string{"198.51.100.10", "198.51.100.11"} {
		req, err := http.NewRequest(http.MethodPost, api.server.URL+"/v1/auth/login", bytes.NewReader([]byte(
			`{"email":"missing@example.com","password":"password123"}`,
		)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", ip)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("anonymous IP %s inherited another IP limit: %d %s", ip, resp.StatusCode, body)
		}
	}
}

func TestLogoutRevokesSessionAndClearsCookie(t *testing.T) {
	api := newAPI(t, 2)
	token := api.user(t, "logout")
	status, body, headers := api.request(t, http.MethodDelete, "/v1/auth/logout", token, nil)
	if status != http.StatusNoContent {
		t.Fatalf("logout: %d %s", status, body)
	}
	cleared := false
	for _, cookie := range (&http.Response{Header: headers}).Cookies() {
		if cookie.Name == "threaden_session" && cookie.MaxAge < 0 && cookie.HttpOnly {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("logout did not clear the session cookie: %v", headers.Values("Set-Cookie"))
	}
	if status, _, _ = api.request(t, http.MethodGet, "/v1/me", token, nil); status != http.StatusUnauthorized {
		t.Fatalf("revoked session still authenticates: %d", status)
	}
}

func TestCookieAuthenticationRejectsCrossSiteMutation(t *testing.T) {
	api := newAPI(t, 2)
	token := api.user(t, "csrf")
	for _, test := range []struct {
		origin string
		want   int
	}{
		{"https://evil.example", http.StatusForbidden},
		{api.server.URL, http.StatusCreated},
	} {
		req, err := http.NewRequest(http.MethodPost, api.server.URL+"/v1/rooms", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "threaden_session", Value: token})
		req.Header.Set("Origin", test.origin)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != test.want {
			t.Fatalf("origin %s: got %d, want %d: %s", test.origin, resp.StatusCode, test.want, body)
		}
	}
}

func TestMessageIdempotencyKeyPreventsDuplicateSend(t *testing.T) {
	api := strictAPI(t)
	owner := api.user(t, "idem-owner")
	status, body, _ := api.request(t, http.MethodPost, "/v1/groups", owner, []byte(`{"name":"Idem","visibility":"public"}`))
	if status != http.StatusCreated {
		t.Fatalf("create group: %d %s", status, body)
	}
	var group struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &group); err != nil {
		t.Fatal(err)
	}
	first := postMessageWithKey(t, api, owner, group.ID, "same-key")
	second := postMessageWithKey(t, api, owner, group.ID, "same-key")
	var firstMessage, secondMessage struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(first, &firstMessage); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second, &secondMessage); err != nil {
		t.Fatal(err)
	}
	if firstMessage.ID == "" || firstMessage.ID != secondMessage.ID {
		t.Fatalf("idempotent id changed: first=%s second=%s", firstMessage.ID, secondMessage.ID)
	}
	status, body, _ = api.request(t, http.MethodGet, "/v1/groups/"+group.ID+"/messages", owner, nil)
	if status != http.StatusOK || bytes.Count(body, []byte("hello once")) != 1 {
		t.Fatalf("duplicate message persisted: %d %s", status, body)
	}
}

func TestMessageReadReceiptUpdatesSender(t *testing.T) {
	api := newAPI(t, 4)
	owner, reader := api.user(t, "receipt-owner"), api.user(t, "receipt-reader")
	groupID := createGroup(t, api, owner, "Receipts")
	if status, _, _ := api.request(t, http.MethodPost, "/v1/groups/"+groupID+"/members", reader, nil); status != http.StatusOK {
		t.Fatalf("join group: %d", status)
	}
	status, body, _ := api.request(t, http.MethodPost, "/v1/groups/"+groupID+"/messages", owner, []byte(`{"body":"read me"}`))
	if status != http.StatusCreated {
		t.Fatalf("send message: %d %s", status, body)
	}
	var message struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &message); err != nil {
		t.Fatal(err)
	}
	status, body, _ = api.request(t, http.MethodGet, "/v1/groups/"+groupID+"/messages", owner, nil)
	if status != http.StatusOK || bytes.Contains(body, []byte(`"read":true`)) {
		t.Fatalf("message should be unread: %d %s", status, body)
	}
	status, body, _ = api.request(t, http.MethodPost, "/v1/groups/"+groupID+"/read", reader, []byte(fmt.Sprintf(`{"message_id":%q}`, message.ID)))
	if status != http.StatusNoContent {
		t.Fatalf("mark read: %d %s", status, body)
	}
	status, body, _ = api.request(t, http.MethodGet, "/v1/groups/"+groupID+"/messages", owner, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"read":true`)) {
		t.Fatalf("message should be read: %d %s", status, body)
	}
}

func postMessageWithKey(t *testing.T, api *testAPI, token, groupID, key string) []byte {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, api.server.URL+"/v1/groups/"+groupID+"/messages", bytes.NewReader([]byte(`{"body":"hello once"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("send with idempotency: %d %s", resp.StatusCode, body)
	}
	return body
}
