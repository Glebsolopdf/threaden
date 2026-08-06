package authorization_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"voice-rooms/internal/groups"
	"voice-rooms/internal/groups/hub"
	"voice-rooms/internal/httpapi"
	"voice-rooms/internal/model"
	"voice-rooms/internal/store"
)

type voiceFake struct{}

func (voiceFake) PublicURL() string                                           { return "ws://voice.test" }
func (voiceFake) JoinToken(string, model.User, time.Duration) (string, error) { return "token", nil }
func (voiceFake) DeleteRoom(context.Context, string) error                    { return nil }
func (voiceFake) RemoveParticipant(context.Context, string, string) error     { return nil }

type api struct {
	server *httptest.Server
	store  *store.Store
}

func newAPI(t *testing.T) *api {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	voice := voiceFake{}
	security := abuse.DefaultConfig()
	security.GroupCreateLimit = abuse.Limit{Capacity: 100, Refill: time.Hour}
	server := httptest.NewServer(httpapi.NewWithSecurity(app.New(st, voice, time.Hour, time.Hour, 8, logger), groups.New(st, voice, hub.New()), st, logger, []string{"https://client.test"}, security))
	t.Cleanup(func() { server.Close(); st.Close() })
	return &api{server, st}
}

func (a *api) request(t *testing.T, method, path, token string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, a.server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, data
}

func tokenFrom(t *testing.T, response *http.Response) string {
	t.Helper()
	for _, cookie := range response.Cookies() {
		if cookie.Name == "threaden_session" {
			return cookie.Value
		}
	}
	t.Fatal("session cookie missing")
	return ""
}
func (a *api) user(t *testing.T, name string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, a.server.URL+"/v1/auth/register", bytes.NewBufferString(fmt.Sprintf(`{"email":%q,"password":"password123"}`, name+"@example.test")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register %s: %d", name, resp.StatusCode)
	}
	return tokenFrom(t, resp)
}
func groupID(t *testing.T, a *api, token, name string) string {
	t.Helper()
	status, body := a.request(t, http.MethodPost, "/v1/groups", token, []byte(fmt.Sprintf(`{"name":%q,"visibility":"public"}`, name)))
	if status != http.StatusCreated {
		t.Fatalf("group: %d %s", status, body)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return result.ID
}
func jsonHasKey(t *testing.T, body []byte, key string) bool {
	t.Helper()
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	var found func(any) bool
	found = func(v any) bool {
		switch item := v.(type) {
		case map[string]any:
			if _, ok := item[key]; ok {
				return true
			}
			for _, child := range item {
				if found(child) {
					return true
				}
			}
		case []any:
			for _, child := range item {
				if found(child) {
					return true
				}
			}
		}
		return false
	}
	return found(value)
}

func TestAuthorizationAndPublicJSON(t *testing.T) {
	a := newAPI(t)
	alice, bob := a.user(t, "alice"), a.user(t, "bob")
	aliceGroup, bobGroup := groupID(t, a, alice, "Alice"), groupID(t, a, bob, "Bob")
	status, message := a.request(t, http.MethodPost, "/v1/groups/"+aliceGroup+"/messages", alice, []byte(`{"body":"private"}`))
	if status != http.StatusCreated || jsonHasKey(t, message, "email") {
		t.Fatalf("message privacy: %d %s", status, message)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(message, &created); err != nil {
		t.Fatal(err)
	}
	if status, body := a.request(t, http.MethodGet, "/v1/groups/"+aliceGroup, bob, nil); status != http.StatusOK || jsonHasKey(t, body, "email") {
		t.Fatalf("group privacy: %d %s", status, body)
	}
	for _, path := range []string{"/v1/groups/" + aliceGroup + "/messages", "/v1/groups/" + aliceGroup + "/profile"} {
		if status, _ = a.request(t, http.MethodGet, path, bob, nil); status != http.StatusForbidden {
			t.Fatalf("foreign read %s: %d", path, status)
		}
	}
	if status, _ = a.request(t, http.MethodPost, "/v1/groups/"+aliceGroup+"/messages", bob, []byte(`{"body":"no"}`)); status != http.StatusForbidden {
		t.Fatalf("foreign send: %d", status)
	}
	if status, _ = a.request(t, http.MethodDelete, "/v1/groups/"+aliceGroup+"/messages/"+created.ID, bob, nil); status != http.StatusForbidden {
		t.Fatalf("foreign delete: %d", status)
	}
	if status, _ = a.request(t, http.MethodDelete, "/v1/groups/"+aliceGroup, bob, nil); status != http.StatusForbidden {
		t.Fatalf("foreign group delete: %d", status)
	}
	if status, _ = a.request(t, http.MethodPost, "/v1/groups/"+bobGroup+"/messages", bob, []byte(fmt.Sprintf(`{"body":"cross","reply_to_id":%q}`, created.ID))); status != http.StatusBadRequest {
		t.Fatalf("cross-group reply: %d", status)
	}
	status, history := a.request(t, http.MethodGet, "/v1/groups/"+aliceGroup+"/messages", alice, nil)
	if status != http.StatusOK || jsonHasKey(t, history, "email") {
		t.Fatalf("history privacy: %d %s", status, history)
	}
	status, room := a.request(t, http.MethodPost, "/v1/rooms", alice, nil)
	if status != http.StatusCreated {
		t.Fatalf("room: %d", status)
	}
	var r struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(room, &r); err != nil {
		t.Fatal(err)
	}
	if status, _ = a.request(t, http.MethodGet, "/v1/rooms/"+r.Code, bob, nil); status != http.StatusNotFound {
		t.Fatalf("foreign room: %d", status)
	}
	status, me := a.request(t, http.MethodGet, "/v1/me", alice, nil)
	if status != http.StatusOK || !jsonHasKey(t, me, "email") {
		t.Fatalf("own profile: %d %s", status, me)
	}
}

func TestSSEMessageDoesNotExposeEmail(t *testing.T) {
	a := newAPI(t)
	alice := a.user(t, "sse-alice")
	group := groupID(t, a, alice, "SSE")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.server.URL+"/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+alice)
	stream, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	if stream.StatusCode != http.StatusOK {
		t.Fatalf("events: %d", stream.StatusCode)
	}
	if status, body := a.request(t, http.MethodPost, "/v1/groups/"+group+"/messages", alice, []byte(`{"body":"event"}`)); status != http.StatusCreated {
		t.Fatalf("message: %d %s", status, body)
	}
	scanner := bufio.NewScanner(stream.Body)
	for scanner.Scan() {
		if strings.TrimPrefix(scanner.Text(), "event: ") != "message_created" {
			continue
		}
		if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "data: ") {
			t.Fatal("message event data missing")
		}
		if jsonHasKey(t, []byte(strings.TrimPrefix(scanner.Text(), "data: ")), "email") {
			t.Fatal("SSE message exposed email")
		}
		return
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	t.Fatal("message event not received")
}
