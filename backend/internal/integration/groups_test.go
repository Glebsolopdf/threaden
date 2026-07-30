package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGroupsPrivacyInviteAndMessages(t *testing.T) {
	api := newAPI(t, 4)
	owner, member, stranger := api.user(t, "owner"), api.user(t, "member"), api.user(t, "stranger")
	status, body, _ := api.request(t, http.MethodPost, "/v1/groups", owner, []byte(`{"name":"Public","visibility":"public"}`))
	if status != http.StatusCreated {
		t.Fatalf("create public: %d %s", status, body)
	}
	var public struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &public); err != nil {
		t.Fatal(err)
	}
	status, body, _ = api.request(t, http.MethodGet, "/v1/discover/groups?q=pub", "", nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(public.ID)) {
		t.Fatalf("discover: %d %s", status, body)
	}
	status, _, _ = api.request(t, http.MethodGet, "/v1/groups/"+public.ID, "", nil)
	if status != http.StatusOK {
		t.Fatalf("public preview: %d", status)
	}
	status, _, _ = api.request(t, http.MethodPost, "/v1/groups/"+public.ID+"/messages", stranger, []byte(`{"body":"blocked"}`))
	if status != http.StatusForbidden {
		t.Fatalf("outsider send: %d", status)
	}
	status, _, _ = api.request(t, http.MethodPost, "/v1/groups/"+public.ID+"/members", member, nil)
	if status != http.StatusOK {
		t.Fatalf("join public: %d", status)
	}
	status, body, _ = api.request(t, http.MethodPost, "/v1/groups/"+public.ID+"/messages", member, []byte(`{"body":"hello group"}`))
	if status != http.StatusCreated || !bytes.Contains(body, []byte("hello group")) {
		t.Fatalf("send: %d %s", status, body)
	}

	status, body, _ = api.request(t, http.MethodPost, "/v1/groups", owner, []byte(`{"name":"Private","visibility":"private"}`))
	if status != http.StatusCreated {
		t.Fatalf("create private: %d %s", status, body)
	}
	var private struct {
		ID     string `json:"id"`
		Invite string `json:"invite_token"`
	}
	if err := json.Unmarshal(body, &private); err != nil {
		t.Fatal(err)
	}
	status, _, _ = api.request(t, http.MethodGet, "/v1/groups/"+private.ID, stranger, nil)
	if status != http.StatusForbidden {
		t.Fatalf("private preview: %d", status)
	}
	status, _, _ = api.request(t, http.MethodPost, "/v1/invites/"+private.Invite+"/join", stranger, nil)
	if status != http.StatusOK {
		t.Fatalf("invite join: %d", status)
	}
	status, _, _ = api.request(t, http.MethodPost, "/v1/invites/missing/join", stranger, nil)
	if status != http.StatusNotFound {
		t.Fatalf("invalid invite: %d", status)
	}
}

func TestGroupProfileAndDeletion(t *testing.T) {
	api := newAPI(t, 4)
	owner, member, removable, stranger := api.user(t, "profile-owner"), api.user(t, "profile-member"), api.user(t, "profile-removable"), api.user(t, "profile-stranger")
	status, body, _ := api.request(t, http.MethodPost, "/v1/groups", owner, []byte(`{"name":"Profile","visibility":"public"}`))
	if status != http.StatusCreated {
		t.Fatalf("create group: %d %s", status, body)
	}
	var group struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &group); err != nil {
		t.Fatal(err)
	}
	ownerID := currentUserID(t, api, owner)
	removableID := currentUserID(t, api, removable)
	if status, _, _ = api.request(t, http.MethodPost, "/v1/groups/"+group.ID+"/members", member, nil); status != http.StatusOK {
		t.Fatalf("join member: %d", status)
	}
	if status, _, _ = api.request(t, http.MethodPost, "/v1/groups/"+group.ID+"/members", removable, nil); status != http.StatusOK {
		t.Fatalf("join removable: %d", status)
	}
	status, body, _ = api.request(t, http.MethodGet, "/v1/groups/"+group.ID+"/profile", member, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"role":"owner"`)) || !bytes.Contains(body, []byte(`"role":"member"`)) {
		t.Fatalf("profile: %d %s", status, body)
	}
	if bytes.Contains(body, []byte(`"description"`)) {
		t.Fatalf("obsolete description returned: %s", body)
	}
	if status, body, _ = api.request(t, http.MethodGet, "/v1/groups/"+group.ID+"/profile", stranger, nil); status != http.StatusOK {
		t.Fatalf("outsider profile: %d", status)
	} else if !bytes.Contains(body, []byte(`"role":"owner"`)) || !bytes.Contains(body, []byte(`"role":"member"`)) {
		t.Fatalf("outsider profile body: %s", body)
	}
	if status, _, _ = api.request(t, http.MethodDelete, "/v1/groups/"+group.ID+"/members/"+removableID, member, nil); status != http.StatusForbidden {
		t.Fatalf("non-owner remove member: %d", status)
	}
	if status, _, _ = api.request(t, http.MethodDelete, "/v1/groups/"+group.ID+"/members/"+ownerID, owner, nil); status != http.StatusForbidden {
		t.Fatalf("owner remove owner: %d", status)
	}
	status, body, _ = api.request(t, http.MethodDelete, "/v1/groups/"+group.ID+"/members/"+removableID, owner, nil)
	if status != http.StatusOK || bytes.Contains(body, []byte(`profile-removable`)) {
		t.Fatalf("owner remove member: %d %s", status, body)
	}
	if status, _, _ = api.request(t, http.MethodDelete, "/v1/groups/"+group.ID+"/members/"+removableID, owner, nil); status != http.StatusNotFound {
		t.Fatalf("repeat remove member: %d", status)
	}
	if status, _, _ = api.request(t, http.MethodDelete, "/v1/groups/"+group.ID+"/members/me", owner, nil); status != http.StatusForbidden {
		t.Fatalf("owner leave: %d", status)
	}
	if status, _, _ = api.request(t, http.MethodDelete, "/v1/groups/"+group.ID+"/members/me", member, nil); status != http.StatusNoContent {
		t.Fatalf("member leave: %d", status)
	}
	if status, _, _ = api.request(t, http.MethodDelete, "/v1/groups/"+group.ID, member, nil); status != http.StatusForbidden {
		t.Fatalf("member delete: %d", status)
	}
	if status, _, _ = api.request(t, http.MethodDelete, "/v1/groups/"+group.ID, owner, nil); status != http.StatusNoContent {
		t.Fatalf("owner delete: %d", status)
	}
	if status, _, _ = api.request(t, http.MethodGet, "/v1/groups/"+group.ID, owner, nil); status != http.StatusNotFound {
		t.Fatalf("deleted group remains: %d", status)
	}
}

func TestGroupAvatarAndDiscoveryBounds(t *testing.T) {
	api := newAPI(t, 4)
	owner := api.user(t, "group-bounds")
	status, body, _ := api.request(t, http.MethodPost, "/v1/groups", owner, []byte(
		`{"name":"Unsafe Avatar","avatar":"data:image/png;base64,AAAA","visibility":"public"}`,
	))
	if status != http.StatusBadRequest || !bytes.Contains(body, []byte(`"validation_error"`)) {
		t.Fatalf("unbounded group avatar accepted: %d %s", status, body)
	}
	for _, path := range []string{"/v1/discover/groups?q=a", "/v1/discover/groups?limit=51", "/v1/discover/groups?offset=1001"} {
		status, body, _ = api.request(t, http.MethodGet, path, "", nil)
		if status != http.StatusBadRequest || !bytes.Contains(body, []byte(`"validation_error"`)) {
			t.Fatalf("invalid discovery bounds accepted for %s: %d %s", path, status, body)
		}
	}
}

func currentUserID(t *testing.T, api *testAPI, token string) string {
	t.Helper()
	status, body, _ := api.request(t, http.MethodGet, "/v1/me", token, nil)
	if status != http.StatusOK {
		t.Fatalf("get user: %d %s", status, body)
	}
	var user struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &user); err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func TestGroupVoiceRoomLimit(t *testing.T) {
	api := newAPI(t, 4)
	owner, member := api.user(t, "voice-limit-owner"), api.user(t, "voice-limit-member")
	status, body, _ := api.request(t, http.MethodPost, "/v1/groups", owner, []byte(`{"name":"Voice Limit","visibility":"public"}`))
	if status != http.StatusCreated {
		t.Fatalf("create group: %d %s", status, body)
	}
	var group struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &group); err != nil {
		t.Fatal(err)
	}
	if status, _, _ = api.request(t, http.MethodPost, "/v1/groups/"+group.ID+"/members", member, nil); status != http.StatusOK {
		t.Fatalf("join member: %d", status)
	}
	if status, _, _ = api.request(t, http.MethodPost, "/v1/groups/"+group.ID+"/voice-rooms", member, []byte(`{"name":"Nope"}`)); status != http.StatusForbidden {
		t.Fatalf("member create voice room: %d", status)
	}
	for i := 1; i <= 5; i++ {
		status, body, _ = api.request(t, http.MethodPost, "/v1/groups/"+group.ID+"/voice-rooms", owner, []byte(`{"name":"Room"}`))
		if status != http.StatusCreated {
			t.Fatalf("create voice room %d: %d %s", i, status, body)
		}
	}
	status, body, _ = api.request(t, http.MethodPost, "/v1/groups/"+group.ID+"/voice-rooms", owner, []byte(`{"name":"Overflow"}`))
	if status != http.StatusConflict || !bytes.Contains(body, []byte(`"voice_room_limit"`)) {
		t.Fatalf("voice room limit: %d %s", status, body)
	}
}

func TestGroupVoiceRoomDeleteRequiresOwner(t *testing.T) {
	api := newAPI(t, 4)
	owner, member := api.user(t, "voice-delete-owner"), api.user(t, "voice-delete-member")
	status, body, _ := api.request(t, http.MethodPost, "/v1/groups", owner, []byte(`{"name":"Voice Delete","visibility":"public"}`))
	if status != http.StatusCreated {
		t.Fatalf("create group: %d %s", status, body)
	}
	var group struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &group); err != nil {
		t.Fatal(err)
	}
	if status, _, _ = api.request(t, http.MethodPost, "/v1/groups/"+group.ID+"/members", member, nil); status != http.StatusOK {
		t.Fatalf("join member: %d", status)
	}
	status, body, _ = api.request(t, http.MethodPost, "/v1/groups/"+group.ID+"/voice-rooms", owner, []byte(`{"name":"General"}`))
	if status != http.StatusCreated {
		t.Fatalf("create voice room: %d %s", status, body)
	}
	var room struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &room); err != nil {
		t.Fatal(err)
	}
	if status, _, _ = api.request(t, http.MethodDelete, "/v1/group-voice-rooms/"+room.ID, member, nil); status != http.StatusForbidden {
		t.Fatalf("member delete voice room: %d", status)
	}
	if status, _, _ = api.request(t, http.MethodDelete, "/v1/group-voice-rooms/"+room.ID, owner, nil); status != http.StatusNoContent {
		t.Fatalf("owner delete voice room: %d", status)
	}
	if status, _, _ = api.request(t, http.MethodPost, "/v1/group-voice-rooms/"+room.ID+"/join", owner, nil); status != http.StatusNotFound {
		t.Fatalf("deleted voice room remains: %d", status)
	}
}
