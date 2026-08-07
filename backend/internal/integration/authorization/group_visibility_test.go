package authorization_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestPublicPreviewAndPrivateJoinBoundary(t *testing.T) {
	a := newAPI(t)
	owner, member := a.user(t, "preview-owner"), a.user(t, "preview-member")
	public := groupID(t, a, owner, "Public Preview")
	if status, body := a.request(t, http.MethodPost, "/v1/groups/"+public+"/members", member, nil); status != http.StatusOK {
		t.Fatalf("join public: %d %s", status, body)
	}
	if status, body := a.request(t, http.MethodPost, "/v1/groups/"+public+"/messages", member, []byte(`{"body":"preview"}`)); status != http.StatusCreated {
		t.Fatalf("send public: %d %s", status, body)
	}
	if status, body := a.request(t, http.MethodGet, "/v1/groups/"+public+"/messages", "", nil); status != http.StatusOK || !bytes.Contains(body, []byte("preview")) {
		t.Fatalf("public preview: %d %s", status, body)
	}
	if status, body := a.request(t, http.MethodGet, "/v1/groups/"+public+"/profile", owner, nil); status != http.StatusOK {
		t.Fatalf("public profile: %d %s", status, body)
	}

	status, body := a.request(t, http.MethodPost, "/v1/groups", owner, []byte(`{"name":"Private Boundary","visibility":"private"}`))
	if status != http.StatusCreated {
		t.Fatalf("private group: %d %s", status, body)
	}
	var private struct {
		ID     string `json:"id"`
		Invite string `json:"invite_token"`
	}
	if err := json.Unmarshal(body, &private); err != nil {
		t.Fatal(err)
	}
	if status, body = a.request(t, http.MethodPost, "/v1/groups/"+private.ID+"/messages", owner, []byte(`{"body":"before"}`)); status != http.StatusCreated {
		t.Fatalf("send before join: %d %s", status, body)
	}
	if status, body = a.request(t, http.MethodPost, "/v1/invites/"+private.Invite+"/join", member, nil); status != http.StatusOK {
		t.Fatalf("join private: %d %s", status, body)
	}
	if status, body = a.request(t, http.MethodPost, "/v1/groups/"+private.ID+"/messages", owner, []byte(`{"body":"after"}`)); status != http.StatusCreated {
		t.Fatalf("send after join: %d %s", status, body)
	}
	if status, body = a.request(t, http.MethodGet, "/v1/groups/"+private.ID+"/messages", member, nil); status != http.StatusOK || bytes.Contains(body, []byte("before")) || !bytes.Contains(body, []byte("after")) {
		t.Fatalf("private history boundary: %d %s", status, body)
	}
}

func TestMembershipMessagePersistsAndSupportsActions(t *testing.T) {
	a := newAPI(t)
	owner, member := a.user(t, "system-owner"), a.user(t, "system-member")
	group := groupID(t, a, owner, "System Messages")
	if status, body := a.request(t, http.MethodPost, "/v1/groups/"+group+"/members", member, nil); status != http.StatusOK {
		t.Fatalf("join: %d %s", status, body)
	}
	status, body := a.request(t, http.MethodGet, "/v1/groups/"+group+"/messages", member, nil)
	if status != http.StatusOK {
		t.Fatalf("messages: %d %s", status, body)
	}
	var messages []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(body, &messages); err != nil {
		t.Fatal(err)
	}
	if len(messages) == 0 || messages[0].Kind != "system" || messages[0].Body != "К чату присоединился участник: system-member" {
		t.Fatalf("membership message missing: %s", body)
	}
	id := messages[0].ID
	if status, body = a.request(t, http.MethodPost, "/v1/groups/"+group+"/messages", member, []byte(fmt.Sprintf(`{"body":"ack","reply_to_id":%q}`, id))); status != http.StatusCreated {
		t.Fatalf("reply: %d %s", status, body)
	}
	if status, body = a.request(t, http.MethodDelete, "/v1/groups/"+group+"/messages/"+id, owner, nil); status != http.StatusNoContent {
		t.Fatalf("delete: %d %s", status, body)
	}
}
