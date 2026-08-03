package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"voice-rooms/internal/model"
	store "voice-rooms/internal/store"
)

func seedGroup(t *testing.T, st *store.Store, now time.Time) string {
	t.Helper()
	owner := model.User{ID: "usr_1", Email: "owner@example.com", DisplayName: "Owner", CreatedAt: now}
	if err := st.CreateUser(ctx(), owner, []byte("p"), [32]byte{}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateGroup(ctx(), store.NewGroup{ID: "grp_1", Visibility: "public", OwnerID: owner.ID, Name: "Rooms", Avatar: "👥", InviteToken: "inv_1"}, now, 3); err != nil {
		t.Fatal(err)
	}
	return owner.ID
}

func ctx() context.Context { return context.Background() }

func TestVoiceRoomsLifecycle(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "voice.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	seedGroup(t, st, now)

	if err := st.CreateVoiceRoom(ctx(), "gvr_1", "grp_1", "Lobby", now, 5); err != nil {
		t.Fatalf("create first room: %v", err)
	}
	if err := st.CreateVoiceRoom(ctx(), "gvr_2", "grp_1", "Quiet", now, 1); !errors.Is(err, store.ErrRoomFull) {
		t.Fatalf("expected room limit, got %v", err)
	}
	room, err := st.VoiceRoom(ctx(), "gvr_1")
	if err != nil || room.ID != "gvr_1" || room.GroupID != "grp_1" || room.Name != "Lobby" {
		t.Fatalf("read room: %+v %v", room, err)
	}
	if _, err := st.VoiceRoom(ctx(), "gvr_missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := st.DeleteVoiceRoom(ctx(), "gvr_missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("delete missing: expected not found, got %v", err)
	}
}

func TestVoiceMembershipMovesBetweenRooms(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "voice.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	owner := seedGroup(t, st, now)

	for _, id := range []string{"gvr_1", "gvr_2"} {
		if err := st.CreateVoiceRoom(ctx(), id, "grp_1", "Room "+id, now, 5); err != nil {
			t.Fatal(err)
		}
	}
	previous, err := st.EnterVoice(ctx(), "gvr_1", owner, now)
	if err != nil || previous != "" {
		t.Fatalf("first entry: previous %q err %v", previous, err)
	}
	previous, err = st.EnterVoice(ctx(), "gvr_2", owner, now)
	if err != nil || previous != "gvr_1" {
		t.Fatalf("move: previous %q err %v", previous, err)
	}
	room, err := st.VoiceRoom(ctx(), "gvr_2")
	if err != nil || room.ParticipantCount != 1 {
		t.Fatalf("room should hold the moved member: %+v %v", room, err)
	}
	if err := st.LeaveVoice(ctx(), "gvr_2", owner); err != nil {
		t.Fatalf("leave: %v", err)
	}
	room, err = st.VoiceRoom(ctx(), "gvr_2")
	if err != nil || room.ParticipantCount != 0 {
		t.Fatalf("room should be empty: %+v %v", room, err)
	}
}
