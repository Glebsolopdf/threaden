package groupchat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	appgroups "voice-rooms/internal/groups"
	"voice-rooms/internal/groups/hub"
	"voice-rooms/internal/model"
	store "voice-rooms/internal/store"
)

type voiceStub struct{}

func (voiceStub) PublicURL() string { return "ws://voice.test" }
func (voiceStub) JoinToken(string, model.User, time.Duration) (string, error) {
	return "token", nil
}
func (voiceStub) DeleteRoom(context.Context, string) error { return nil }
func (voiceStub) RemoveParticipant(context.Context, string, string) error {
	return nil
}

type recorder struct {
	status int
	body   string
	err    error
}

type harness struct {
	st       *store.Store
	handler  *Handler
	user     model.User
	groupID  string
	last     recorder
	writeErr int
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "handler.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	now := time.Unix(1_700_000_000, 0).UTC()
	user := model.User{ID: "usr_1", Email: "a@example.com", DisplayName: "A", CreatedAt: now}
	if err := st.CreateUser(context.Background(), user, []byte("p"), [32]byte{}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateGroup(context.Background(), store.NewGroup{ID: "grp_1", Visibility: "public", OwnerID: user.ID, Name: "Rooms", Avatar: "👥", InviteToken: "inv_1"}, now, 3); err != nil {
		t.Fatal(err)
	}
	h := &harness{st: st, user: user, groupID: "grp_1"}
	h.handler = New(appgroups.New(st, voiceStub{}, hub.New()), Hooks{
		WriteJSON: func(_ http.ResponseWriter, status int, v any) { h.record(status, v) },
		WriteError: func(_ http.ResponseWriter, r *http.Request, status int, code, msg string) {
			h.record(status, map[string]string{"error": code})
		},
		DecodeJSON:   func(r *http.Request, dst any) error { return json.NewDecoder(r.Body).Decode(dst) },
		CurrentUser:  func(*http.Request) model.User { return h.user },
		OptionalUser: func(*http.Request) *model.User { u := h.user; return &u },
		WriteGroupError: func(_ http.ResponseWriter, _ *http.Request, e error) {
			h.last = recorder{status: 0, err: e}
			h.writeErr++
		},
	})
	return h
}

func (h *harness) record(status int, v any) {
	body, _ := json.Marshal(v)
	h.last = recorder{status: status, body: string(body)}
}

func (h *harness) do(t *testing.T, method, path string, body string) {
	t.Helper()
	h.last = recorder{}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r := chi.NewRouter()
	r.Get("/groups/{id}/messages", h.handler.Messages)
	r.Post("/groups/{id}/messages", h.handler.Send)
	r.Delete("/groups/{id}/messages/{messageID}", h.handler.Delete)
	r.Post("/groups/{id}/typing", h.handler.Typing)
	r.Post("/groups/{id}/read", h.handler.Read)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if h.last.status == 0 {
		h.last = recorder{status: rr.Code, err: h.last.err}
	}
}

func TestMessagesRejectsBadLimit(t *testing.T) {
	h := newHarness(t)
	h.do(t, http.MethodGet, "/groups/"+h.groupID+"/messages?limit=0", "")
	if h.last.status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", h.last.status)
	}
}

func TestMessagesReturnsList(t *testing.T) {
	h := newHarness(t)
	h.do(t, http.MethodGet, "/groups/"+h.groupID+"/messages", "")
	if h.last.status != http.StatusOK {
		t.Fatalf("expected 200, got %d", h.last.status)
	}
}

func TestSendRejectsInvalidJSON(t *testing.T) {
	h := newHarness(t)
	h.do(t, http.MethodPost, "/groups/"+h.groupID+"/messages", "{")
	if h.last.status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", h.last.status)
	}
}

func TestSendCreatesMessage(t *testing.T) {
	h := newHarness(t)
	h.do(t, http.MethodPost, "/groups/"+h.groupID+"/messages", `{"body":"hello"}`)
	if h.last.status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", h.last.status)
	}
	h.do(t, http.MethodGet, "/groups/"+h.groupID+"/messages", "")
	var messages []model.GroupMessage
	if err := json.Unmarshal([]byte(h.last.body), &messages); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Body != "hello" {
		t.Fatalf("message not persisted: %+v", messages)
	}
}

func TestSendReplyAndDeleteMessage(t *testing.T) {
	h := newHarness(t)
	h.do(t, http.MethodPost, "/groups/"+h.groupID+"/messages", `{"body":"original"}`)
	var original model.GroupMessage
	if err := json.Unmarshal([]byte(h.last.body), &original); err != nil {
		t.Fatal(err)
	}
	h.do(t, http.MethodPost, "/groups/"+h.groupID+"/messages", `{"body":"answer","reply_to_id":"`+original.ID+`"}`)
	if h.last.status != http.StatusCreated {
		t.Fatalf("reply: expected 201, got %d", h.last.status)
	}
	var reply model.GroupMessage
	if err := json.Unmarshal([]byte(h.last.body), &reply); err != nil {
		t.Fatal(err)
	}
	if reply.ReplyTo == nil || reply.ReplyTo.ID != original.ID {
		t.Fatalf("reply reference missing: %+v", reply.ReplyTo)
	}
	h.do(t, http.MethodDelete, "/groups/"+h.groupID+"/messages/"+original.ID, "")
	if h.last.status != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", h.last.status)
	}
}

func TestDeleteRequiresAuthorOrOwner(t *testing.T) {
	h := newHarness(t)
	now := h.user.CreatedAt
	owner := h.user
	other := model.User{ID: "usr_2", Email: "b@example.com", DisplayName: "B", CreatedAt: now}
	if err := h.st.CreateUser(context.Background(), other, []byte("p"), [32]byte{1}); err != nil {
		t.Fatal(err)
	}
	if err := h.st.JoinGroup(context.Background(), h.groupID, other.ID, now); err != nil {
		t.Fatal(err)
	}
	h.do(t, http.MethodPost, "/groups/"+h.groupID+"/messages", `{"body":"mine"}`)
	var sent model.GroupMessage
	if err := json.Unmarshal([]byte(h.last.body), &sent); err != nil {
		t.Fatal(err)
	}
	h.user = other
	h.do(t, http.MethodDelete, "/groups/"+h.groupID+"/messages/"+sent.ID, "")
	if h.writeErr == 0 || !errors.Is(h.last.err, appgroups.ErrForbidden) {
		t.Fatalf("non-author delete: expected forbidden error, got err=%v status=%d writeErr=%d", h.last.err, h.last.status, h.writeErr)
	}
	h.do(t, http.MethodPost, "/groups/"+h.groupID+"/messages", `{"body":"theirs"}`)
	var theirs model.GroupMessage
	if err := json.Unmarshal([]byte(h.last.body), &theirs); err != nil {
		t.Fatal(err)
	}
	h.user = owner
	h.writeErr = 0
	h.do(t, http.MethodDelete, "/groups/"+h.groupID+"/messages/"+theirs.ID, "")
	if h.writeErr != 0 || h.last.status != http.StatusNoContent {
		t.Fatalf("owner delete: expected 204, got %d err %v", h.last.status, h.last.err)
	}
}

func TestReplyToDeletedMessageFails(t *testing.T) {
	h := newHarness(t)
	h.do(t, http.MethodPost, "/groups/"+h.groupID+"/messages", `{"body":"original"}`)
	var original model.GroupMessage
	if err := json.Unmarshal([]byte(h.last.body), &original); err != nil {
		t.Fatal(err)
	}
	h.do(t, http.MethodDelete, "/groups/"+h.groupID+"/messages/"+original.ID, "")
	if h.last.status != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", h.last.status)
	}
	h.do(t, http.MethodPost, "/groups/"+h.groupID+"/messages", `{"body":"answer","reply_to_id":"`+original.ID+`"}`)
	if h.writeErr == 0 || !errors.Is(h.last.err, appgroups.ErrNotFound) {
		t.Fatalf("reply to deleted message: expected not-found error, got %v", h.last.err)
	}
}

func TestSendToMissingGroupRunsWriteGroupError(t *testing.T) {
	h := newHarness(t)
	h.do(t, http.MethodPost, "/groups/grp_missing/messages", `{"body":"hello"}`)
	if h.writeErr == 0 {
		t.Fatal("expected WriteGroupError to be called")
	}
}

func TestTypingAndReadSucceed(t *testing.T) {
	h := newHarness(t)
	h.do(t, http.MethodPost, "/groups/"+h.groupID+"/messages", `{"body":"hello"}`)
	if h.last.status != http.StatusCreated {
		t.Fatalf("send: expected 201, got %d", h.last.status)
	}
	var sent model.GroupMessage
	if err := json.Unmarshal([]byte(h.last.body), &sent); err != nil {
		t.Fatal(err)
	}
	h.do(t, http.MethodPost, "/groups/"+h.groupID+"/typing", `{"active":true}`)
	if h.last.status != http.StatusNoContent {
		t.Fatalf("typing: expected 204, got %d", h.last.status)
	}
	h.do(t, http.MethodPost, "/groups/"+h.groupID+"/read", `{"message_id":"`+sent.ID+`"}`)
	if h.last.status != http.StatusNoContent {
		t.Fatalf("read: expected 204, got %d", h.last.status)
	}
}
