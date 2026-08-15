package account_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"voice-rooms/internal/attachments"
	attachmentaccount "voice-rooms/internal/attachments/account"
	accountapi "voice-rooms/internal/httpapi/account"
	"voice-rooms/internal/model"
	"voice-rooms/internal/store/sqlite"
)

func TestDeleteAllEndpointIsIdempotentAndCancelable(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "http.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(id,email,display_name,avatar,password_hash,token_hash,created_at,last_seen_at) VALUES('u','u@example.com','U','',X'',zeroblob(32),1,1)`); err != nil {
		t.Fatal(err)
	}
	service := &attachmentaccount.Service{DB: db, Limits: attachments.Limits{MaxUserStoredBytes: 50}}
	h := accountapi.New(service, accountapi.Hooks{
		WriteJSON: func(w http.ResponseWriter, status int, body any) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(body)
		},
		WriteError: func(w http.ResponseWriter, _ *http.Request, status int, code, message string) {
			http.Error(w, code+": "+message, status)
		},
		CurrentUser: func(*http.Request) model.User { return model.User{ID: "u"} },
	})
	first := httptest.NewRecorder()
	h.ScheduleDelete(first, httptest.NewRequest(http.MethodPost, "/v1/account/attachments/delete-all", nil))
	if first.Code != http.StatusAccepted {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	h.ScheduleDelete(second, httptest.NewRequest(http.MethodPost, "/v1/account/attachments/delete-all", nil))
	if second.Code != http.StatusAccepted || second.Body.String() != first.Body.String() {
		t.Fatalf("second response differs: first=%s second=%s", first.Body.String(), second.Body.String())
	}
	cancel := httptest.NewRecorder()
	h.CancelDelete(cancel, httptest.NewRequest(http.MethodDelete, "/v1/account/attachments/delete-all", nil))
	if cancel.Code != http.StatusNoContent {
		t.Fatalf("cancel status=%d body=%s", cancel.Code, cancel.Body.String())
	}
}
