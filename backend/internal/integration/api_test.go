package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"voice-rooms/internal/app"
	appgroups "voice-rooms/internal/groups"
	"voice-rooms/internal/httpapi"
	"voice-rooms/internal/model"
	"voice-rooms/internal/store"
)

type voiceFake struct{}

func (voiceFake) PublicURL() string { return "ws://voice.test" }
func (voiceFake) JoinToken(string, model.User, time.Duration) (string, error) {
	return "signed-livekit-jwt", nil
}
func (voiceFake) DeleteRoom(context.Context, string) error                { return nil }
func (voiceFake) RemoveParticipant(context.Context, string, string) error { return nil }

type testAPI struct {
	server *httptest.Server
	store  *store.Store
}

func newAPI(t *testing.T, maxMembers int) *testAPI {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "integration.db"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := app.New(st, voiceFake{}, time.Hour, 15*time.Minute, maxMembers, logger)
	groupService := appgroups.New(st, voiceFake{}, appgroups.NewHub())
	server := httptest.NewServer(httpapi.New(service, groupService, st, logger, []string{"https://client.test"}))
	t.Cleanup(func() {
		server.Close()
		st.Close()
	})
	return &testAPI{server: server, store: st}
}

func (a *testAPI) request(t *testing.T, method, path, token string, body []byte) (int, []byte, http.Header) {
	t.Helper()
	return a.requestWithType(t, method, path, token, body, "application/json")
}

func (a *testAPI) requestWithType(
	t *testing.T,
	method, path, token string,
	body []byte,
	contentType string,
) (int, []byte, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, a.server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil && contentType != "" {
		req.Header.Set("Content-Type", contentType)
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
	return resp.StatusCode, data, resp.Header
}

func (a *testAPI) user(t *testing.T, name string) string {
	t.Helper()
	status, body, headers := a.request(t, http.MethodPost, "/v1/auth/register", "", []byte(
		fmt.Sprintf(`{"email":%q,"password":"password123"}`, name+"@example.com"),
	))
	if status != http.StatusCreated {
		t.Fatalf("create user: %d %s", status, body)
	}
	if bytes.Contains(body, []byte("session_token")) {
		t.Fatalf("session token leaked in response: %s", body)
	}
	return sessionTokenFromHeaders(t, headers)
}

func TestAPILifecycleValidationAndCORS(t *testing.T) {
	api := newAPI(t, 2)
	status, body, _ := api.request(t, http.MethodPost, "/v1/auth/register", "", []byte(
		`{"email":"gleb@example.com","password":"password123","unknown":true}`,
	))
	if status != http.StatusBadRequest || !bytes.Contains(body, []byte(`"invalid_json"`)) {
		t.Fatalf("strict JSON validation: %d %s", status, body)
	}
	status, body, _ = api.request(t, http.MethodPost, "/v1/auth/register", "", bytes.Repeat([]byte("x"), 11<<20))
	if status != http.StatusRequestEntityTooLarge || !bytes.Contains(body, []byte(`"body_too_large"`)) {
		t.Fatalf("body limit: %d %s", status, body)
	}
	status, body, _ = api.request(t, http.MethodPost, "/v1/auth/register", "", []byte(
		`{"email":"short@example.com","password":"abc"}`,
	))
	if status != http.StatusBadRequest || !bytes.Contains(body, []byte(`"validation_error"`)) {
		t.Fatalf("short password validation: %d %s", status, body)
	}
	if status, _, _ = api.request(t, http.MethodGet, "/v1/me", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", status)
	}

	ownerToken := api.user(t, "Owner")
	status, body, _ = api.request(t, http.MethodPost, "/v1/auth/login", "", []byte(
		`{"email":"owner@example.com","password":"badpass123"}`,
	))
	if status != http.StatusUnauthorized {
		t.Fatalf("bad login: %d %s", status, body)
	}
	ownerToken = login(t, api, "owner@example.com")
	status, body, _ = updateProfile(t, api, ownerToken, "Owner Name", []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"))
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"display_name":"Owner Name"`)) ||
		!bytes.Contains(body, []byte(`data:image/jpeg;base64`)) {
		t.Fatalf("profile upload: %d %s", status, body)
	}
	status, body, _ = updateProfile(t, api, ownerToken, "Big Avatar", bigPNG(t))
	if status != http.StatusOK || !bytes.Contains(body, []byte(`data:image/jpeg;base64`)) {
		t.Fatalf("large avatar conversion: %d %s", status, body)
	}
	status, body, _ = api.request(t, http.MethodDelete, "/v1/me/avatar", ownerToken, nil)
	if status != http.StatusOK || bytes.Contains(body, []byte(`"avatar"`)) {
		t.Fatalf("delete avatar: %d %s", status, body)
	}
	memberToken := api.user(t, "Member")
	status, body, _ = api.request(t, http.MethodPost, "/v1/rooms", ownerToken, nil)
	if status != http.StatusCreated {
		t.Fatalf("create room: %d %s", status, body)
	}
	var room struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &room); err != nil {
		t.Fatal(err)
	}
	status, body, _ = api.request(t, http.MethodPost, "/v1/rooms/"+room.Code+"/join", memberToken, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"signed-livekit-jwt"`)) {
		t.Fatalf("join room: %d %s", status, body)
	}
	status, body, _ = api.request(t, http.MethodDelete, "/v1/rooms/"+room.Code, memberToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("non-owner delete: %d %s", status, body)
	}
	status, _, _ = api.request(t, http.MethodDelete, "/v1/rooms/"+room.Code+"/members/me", memberToken, nil)
	if status != http.StatusNoContent {
		t.Fatalf("leave: %d", status)
	}
	status, body, _ = api.request(t, http.MethodDelete, "/v1/rooms/"+room.Code, ownerToken, nil)
	if status != http.StatusNoContent {
		t.Fatalf("owner delete: %d %s", status, body)
	}
	status, _, _ = api.request(t, http.MethodGet, "/v1/rooms/"+room.Code, ownerToken, nil)
	if status != http.StatusNotFound {
		t.Fatalf("deleted room remains: %d", status)
	}

	req, _ := http.NewRequest(http.MethodGet, api.server.URL+"/healthz", nil)
	req.Header.Set("Origin", "https://client.test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK ||
		resp.Header.Get("Access-Control-Allow-Origin") != "https://client.test" {
		t.Fatalf("health/CORS response: %d %v", resp.StatusCode, resp.Header)
	}
	if status, _, _ := api.request(t, http.MethodGet, "/readyz", "", nil); status != http.StatusOK {
		t.Fatalf("readyz: %d", status)
	}
}

func TestDeleteProfile(t *testing.T) {
	api := newAPI(t, 2)
	token := api.user(t, "Gone")
	status, body, _ := api.request(t, http.MethodDelete, "/v1/me", token, nil)
	if status != http.StatusNoContent {
		t.Fatalf("delete profile: %d %s", status, body)
	}
	status, _, _ = api.request(t, http.MethodGet, "/v1/me", token, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("deleted profile still authenticates: %d", status)
	}
}

func login(t *testing.T, api *testAPI, email string) string {
	t.Helper()
	status, body, headers := api.request(t, http.MethodPost, "/v1/auth/login", "", []byte(
		fmt.Sprintf(`{"email":%q,"password":"password123"}`, email),
	))
	if status != http.StatusOK {
		t.Fatalf("login: %d %s", status, body)
	}
	if bytes.Contains(body, []byte("session_token")) {
		t.Fatalf("session token leaked in login response: %s", body)
	}
	return sessionTokenFromHeaders(t, headers)
}

func updateProfile(t *testing.T, api *testAPI, token, name string, avatar []byte) (int, []byte, http.Header) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("display_name", name); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("avatar", "avatar.gif")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(avatar); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return api.requestWithType(t, http.MethodPatch, "/v1/me", token, body.Bytes(), writer.FormDataContentType())
}

func TestConcurrentRoomLimit(t *testing.T) {
	api := newAPI(t, 3)
	owner := api.user(t, "Owner")
	status, body, _ := api.request(t, http.MethodPost, "/v1/rooms", owner, nil)
	if status != http.StatusCreated {
		t.Fatalf("create room: %d %s", status, body)
	}
	var room struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &room); err != nil {
		t.Fatal(err)
	}
	tokens := make([]string, 8)
	for i := range tokens {
		tokens[i] = api.user(t, fmt.Sprintf("Member%d", i))
	}
	var wg sync.WaitGroup
	results := make(chan int, len(tokens))
	for _, token := range tokens {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodPost, api.server.URL+"/v1/rooms/"+room.Code+"/join", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				results <- 0
				return
			}
			resp.Body.Close()
			results <- resp.StatusCode
		}()
	}
	wg.Wait()
	close(results)
	ok, full := 0, 0
	for status := range results {
		if status == http.StatusOK {
			ok++
		}
		if status == http.StatusConflict {
			full++
		}
	}
	if ok != 2 || full != len(tokens)-2 {
		t.Fatalf("capacity results: joined=%d full=%d", ok, full)
	}
	status, body, _ = api.request(t, http.MethodGet, "/v1/rooms/"+room.Code, owner, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"participant_count":3`)) {
		t.Fatalf("wrong final participant count: %d %s", status, body)
	}
}
