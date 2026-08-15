package accountblock_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"voice-rooms/internal/abuse"
	"voice-rooms/internal/app"
	appgroups "voice-rooms/internal/groups"
	"voice-rooms/internal/groups/hub"
	"voice-rooms/internal/httpapi"
	"voice-rooms/internal/model"
	"voice-rooms/internal/store"
)

type api struct {
	server *httptest.Server
	store  *store.Store
}
type voiceFake struct{}

func (voiceFake) PublicURL() string                                           { return "ws://voice.test" }
func (voiceFake) JoinToken(string, model.User, time.Duration) (string, error) { return "token", nil }
func (voiceFake) DeleteRoom(context.Context, string) error                    { return nil }
func (voiceFake) RemoveParticipant(context.Context, string, string) error     { return nil }

func newAPI(t *testing.T) *api {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "block.db"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	voice := voiceFake{}
	service := app.New(st, voice, time.Hour, 15*time.Minute, 4, logger)
	security := abuse.DefaultConfig()
	security.GroupCreateLimit = abuse.Limit{Capacity: 100, Refill: time.Hour}
	groups := appgroups.New(st, voice, hub.New())
	server := httptest.NewServer(httpapi.NewWithSecurity(service, groups, st, logger, []string{"https://client.test"}, security))
	t.Cleanup(func() { server.Close(); st.Close() })
	return &api{server: server, store: st}
}

func (a *api) request(t *testing.T, method, path, token string, body []byte, contentType string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, a.server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, data
}

func (a *api) user(t *testing.T, name string) string {
	body := []byte(`{"email":"` + name + `@example.com","password":"password123"}`)
	req, err := http.NewRequest(http.MethodPost, a.server.URL+"/v1/auth/register", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("register: %d %s", resp.StatusCode, data)
	}
	return strings.TrimPrefix(strings.Split(resp.Header.Get("Set-Cookie"), ";")[0], "threaden_session=")
}

func TestTemporaryBlockRejectsAllAuthenticatedRequests(t *testing.T) {
	api := newAPI(t)
	blockedToken := api.user(t, "blocked-everywhere")
	otherToken := api.user(t, "unblocked-everywhere")
	status, body := api.request(t, http.MethodPost, "/v1/groups", blockedToken, []byte(`{"name":"Block Matrix","avatar":"","visibility":"public"}`), "application/json")
	if status != http.StatusCreated {
		t.Fatalf("create group: %d %s", status, body)
	}
	var group struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &group); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	user, err := api.store.UserBySessionHash(context.Background(), sha256.Sum256([]byte(blockedToken)), now, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := api.store.SetAccountBlock(context.Background(), user.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	requests := []struct {
		name, method, path, contentType string
		body                            []byte
	}{
		{name: "me", method: http.MethodGet, path: "/v1/me"},
		{name: "chat history", method: http.MethodGet, path: "/v1/groups/" + group.ID + "/messages"},
		{name: "profile update", method: http.MethodPatch, path: "/v1/me", contentType: "multipart/form-data", body: []byte("blocked")},
		{name: "multipart message", method: http.MethodPost, path: "/v1/groups/" + group.ID + "/messages", contentType: "multipart/form-data", body: []byte("blocked")},
		{name: "logout", method: http.MethodDelete, path: "/v1/auth/logout"},
	}
	for _, request := range requests {
		t.Run(request.name, func(t *testing.T) {
			status, body := api.request(t, request.method, request.path, blockedToken, request.body, request.contentType)
			if status != http.StatusTooManyRequests || !bytes.Contains(body, []byte(`"account_blocked"`)) {
				t.Fatalf("blocked request: status=%d body=%s", status, body)
			}
		})
	}
	cookieRequest, err := http.NewRequest(http.MethodGet, api.server.URL+"/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	cookieRequest.AddCookie(&http.Cookie{Name: "threaden_session", Value: blockedToken})
	cookieResponse, err := http.DefaultClient.Do(cookieRequest)
	if err != nil {
		t.Fatal(err)
	}
	cookieBody, err := io.ReadAll(cookieResponse.Body)
	cookieResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if cookieResponse.StatusCode != http.StatusTooManyRequests || !bytes.Contains(cookieBody, []byte(`"account_blocked"`)) {
		t.Fatalf("blocked cookie request: status=%d body=%s", cookieResponse.StatusCode, cookieBody)
	}
	if status, _ := api.request(t, http.MethodGet, "/v1/me", otherToken, nil, ""); status != http.StatusOK {
		t.Fatalf("unblocked account rejected: %d", status)
	}
	if status, _ := api.request(t, http.MethodGet, "/v1/groups/"+group.ID+"/messages", "", nil, ""); status != http.StatusOK {
		t.Fatalf("anonymous public read rejected: %d", status)
	}
	if err := api.store.SetAccountBlock(context.Background(), user.ID, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if status, _ := api.request(t, http.MethodGet, "/v1/me", blockedToken, nil, ""); status != http.StatusOK {
		t.Fatalf("expired block did not release account: %d", status)
	}
}
