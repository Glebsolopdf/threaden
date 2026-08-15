package attachments_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"voice-rooms/internal/abuse"
	"voice-rooms/internal/app"
	"voice-rooms/internal/attachments"
	attachmentstorage "voice-rooms/internal/attachments/storage"
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
	st, err := store.Open(filepath.Join(t.TempDir(), "audio.db"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	voice := voiceFake{}
	service := app.New(st, voice, time.Hour, 15*time.Minute, 4, logger)
	security := abuse.DefaultConfig()
	security.GroupCreateLimit = abuse.Limit{Capacity: 100, Refill: time.Hour}
	limits := attachments.Limits{MaxInputMediaBytes: 1 << 20, MaxArchiveBytes: 1 << 20, MaxOutputMediaBytes: 1 << 20, MaxFilesPerMessage: 3, MaxUserStoredBytes: 50 << 20, MaxUserDailyBytes: 20 << 20, MaxTotalBytes: 100 << 20, Retention: 72 * time.Hour}
	attachmentService := &attachmentstorage.Service{Root: t.TempDir(), Limits: limits, Processor: attachments.Processor{MaxInputMedia: 1 << 20, MaxArchive: 1 << 20, MaxOutputMedia: 1 << 20}, DB: st}
	groups := appgroups.New(st, voice, hub.New())
	server := httptest.NewServer(httpapi.NewWithOptions(service, groups, st, logger, httpapi.Options{Security: security, Attachments: attachmentService}))
	t.Cleanup(func() { server.Close(); st.Close() })
	return &api{server: server, store: st}
}

func (a *api) request(t *testing.T, method, path, token string, body []byte, contentType string) (*http.Response, []byte) {
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
	return resp, data
}

func (a *api) user(t *testing.T) string {
	resp, body := a.request(t, http.MethodPost, "/v1/auth/register", "", []byte(`{"email":"audio@example.com","password":"password123"}`), "application/json")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d %s", resp.StatusCode, body)
	}
	return strings.TrimPrefix(strings.Split(resp.Header.Get("Set-Cookie"), ";")[0], "threaden_session=")
}

func TestAudioMultipartMessageUsesAttachmentPipeline(t *testing.T) {
	api := newAPI(t)
	token := api.user(t)
	resp, body := api.request(t, http.MethodPost, "/v1/groups", token, []byte(`{"name":"Audio Group","avatar":"","visibility":"public"}`), "application/json")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create group: %d %s", resp.StatusCode, body)
	}
	var group struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &group); err != nil {
		t.Fatal(err)
	}

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, err := writer.CreateFormFile("files[]", "voice.wav")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("RIFF\x18\x00\x00\x00WAVEfmt audio"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	resp, body = api.request(t, http.MethodPost, "/v1/groups/"+group.ID+"/messages", token, form.Bytes(), writer.FormDataContentType())
	if resp.StatusCode != http.StatusCreated || !bytes.Contains(body, []byte(`"kind":"audio"`)) {
		t.Fatalf("audio message: status=%d body=%s", resp.StatusCode, body)
	}
	var kind string
	var size, created, expires int64
	if err := api.store.QueryRowContext(t.Context(), `SELECT kind,size,created_at,expires_at FROM attachments WHERE kind='audio'`).Scan(&kind, &size, &created, &expires); err != nil {
		t.Fatal(err)
	}
	if kind != "audio" || size <= 0 || expires-created != int64(72*time.Hour/time.Second) {
		t.Fatalf("unexpected audio metadata: kind=%q size=%d created=%d expires=%d", kind, size, created, expires)
	}
}
