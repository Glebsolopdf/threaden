package store_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"voice-rooms/internal/model"
	store "voice-rooms/internal/store"
)

func TestStoreLifecycleAndTokenHash(t *testing.T) {
	ctx := context.Background()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	now := time.Unix(1_700_000_000, 0).UTC()
	token := "raw-session-token"
	owner := model.User{
		ID: "usr_owner", Email: "owner@example.com", DisplayName: "Owner", Avatar: "🦊",
		CreatedAt: now,
	}
	if err := st.CreateUser(ctx, owner, []byte("hashed-password"), sha256.Sum256([]byte(token))); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserByTokenHash(ctx, sha256.Sum256([]byte(token))); err != nil {
		t.Fatalf("authenticate stored user: %v", err)
	}
	_, passwordHash, err := st.UserByEmail(ctx, "owner@example.com")
	if err != nil || string(passwordHash) != "hashed-password" {
		t.Fatalf("read password hash: %q %v", passwordHash, err)
	}

	if err := st.InsertRoom(ctx, "AB12", owner.ID, now, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertRoom(ctx, "AB12", owner.ID, now, time.Hour); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("expected database code collision, got %v", err)
	}
	second := model.User{
		ID: "usr_second", Email: "second@example.com", DisplayName: "Second", Avatar: "🐼",
		CreatedAt: now,
	}
	if err := st.CreateUser(ctx, second, []byte("hashed-second"), sha256.Sum256([]byte("second"))); err != nil {
		t.Fatal(err)
	}
	if _, err := st.JoinRoom(ctx, "AB12", second.ID, now, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := st.JoinRoom(ctx, "AB12", second.ID, now, 2); err != nil {
		t.Fatalf("repeat join must be idempotent: %v", err)
	}
	room, err := st.GetRoom(ctx, "AB12", now, 2)
	if err != nil || room.ParticipantCount != 2 {
		t.Fatalf("unexpected room: %+v, %v", room, err)
	}
	if err := st.LeaveRoom(ctx, "AB12", second.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteRoom(ctx, "AB12", second.ID, now, func() error { return nil }); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("non-owner deletion: %v", err)
	}
	called := false
	if err := st.DeleteRoom(ctx, "AB12", owner.ID, now, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("room terminator was not called")
	}
}

func TestEmptyRoomIsCleanupCandidateAfterGrace(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "empty-room.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	owner := model.User{ID: "usr_empty_owner", Email: "empty-owner@example.com", DisplayName: "Owner", CreatedAt: now}
	if err := st.CreateUser(ctx, owner, []byte("hash"), sha256.Sum256([]byte("owner"))); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertRoom(ctx, "CD34", owner.ID, now, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := st.LeaveRoom(ctx, "CD34", owner.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if codes, err := st.EmptyRoomCodes(ctx, now); err != nil || len(codes) != 0 {
		t.Fatalf("room selected before two minutes: %v err=%v", codes, err)
	}
	codes, err := st.EmptyRoomCodes(ctx, now.Add(time.Minute))
	if err != nil || len(codes) != 1 || codes[0] != "CD34" {
		t.Fatalf("room not selected after grace: %v err=%v", codes, err)
	}
	if err := st.DeleteEmptyRoom(ctx, "CD34", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetRoom(ctx, "CD34", now.Add(3*time.Minute), 2); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("empty room remains: %v", err)
	}
}

func TestDeleteInactiveUsers(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "inactive.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	old := model.User{ID: "usr_old", Email: "old@example.com", DisplayName: "Old", Avatar: "data:image/jpeg;base64,old", CreatedAt: now}
	older := model.User{ID: "usr_older", Email: "older@example.com", DisplayName: "Older", Avatar: "", CreatedAt: now.Add(-time.Hour)}
	active := model.User{ID: "usr_active", Email: "active@example.com", DisplayName: "Active", Avatar: "🙂", CreatedAt: now}
	if err := st.CreateUser(ctx, old, []byte("hash"), sha256.Sum256([]byte("old"))); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, older, []byte("hash"), sha256.Sum256([]byte("older"))); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, active, []byte("hash"), sha256.Sum256([]byte("active"))); err != nil {
		t.Fatal(err)
	}
	if err := st.TouchUser(ctx, active.ID, now.Add(8*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	count, err := st.DeleteInactiveUsersBatch(ctx, now.Add(7*24*time.Hour), 1)
	if err != nil || count != 1 {
		t.Fatalf("batch inactive delete count=%d err=%v", count, err)
	}
	if _, err := st.UserByTokenHash(ctx, sha256.Sum256([]byte("older"))); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("oldest inactive user remains: %v", err)
	}
	if err := st.DeleteInactiveUsers(ctx, now.Add(7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserByTokenHash(ctx, sha256.Sum256([]byte("old"))); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("inactive user remains: %v", err)
	}
	if _, err := st.UserByTokenHash(ctx, sha256.Sum256([]byte("active"))); err != nil {
		t.Fatalf("active user deleted: %v", err)
	}
}

func TestInactiveGroupScheduleAndDelete(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "groups-cleanup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	owner := model.User{ID: "usr_cleanup", Email: "cleanup@example.com", DisplayName: "Cleanup", Avatar: "", CreatedAt: now}
	if err := st.CreateUser(ctx, owner, []byte("hash"), sha256.Sum256([]byte("cleanup"))); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateGroup(ctx, store.NewGroup{
		ID: "grp_cleanup", Visibility: "public", OwnerID: owner.ID,
		Name: "Cleanup", Avatar: "", InviteToken: "inv_cleanup",
	}, now.Add(-8*24*time.Hour), 0); err != nil {
		t.Fatal(err)
	}
	if err := st.AddMessage(ctx, model.GroupMessage{ID: "msg_old", GroupID: "grp_cleanup", Author: owner, Body: "old", CreatedAt: now.Add(-9 * 24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if count, err := st.DeleteOldestMessages(ctx, now.Add(-24*time.Hour), 10); err != nil || count != 1 {
		t.Fatalf("old message delete count=%d err=%v", count, err)
	}
	candidates, err := st.InactiveGroupCandidates(ctx, now.Add(-7*24*time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ID != "grp_cleanup" {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
	count, err := st.ScheduleInactiveGroups(ctx, now.Add(-7*24*time.Hour), now.Add(time.Hour), 10)
	if err != nil || count != 1 {
		t.Fatalf("schedule count=%d err=%v", count, err)
	}
	if deleted, err := st.DeleteScheduledGroups(ctx, now, 10); err != nil || len(deleted) != 0 {
		t.Fatalf("deleted before grace: %+v err=%v", deleted, err)
	}
	deleted, err := st.DeleteScheduledGroups(ctx, now.Add(2*time.Hour), 10)
	if err != nil || len(deleted) != 1 || deleted[0] != "grp_cleanup" {
		t.Fatalf("deleted after grace: %+v err=%v", deleted, err)
	}
	if _, err := st.Group(ctx, "grp_cleanup"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("group remains after cleanup: %v", err)
	}
}

func TestAddMessageDuplicateIDIsNotNotFound(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "dupmsg.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	author := model.User{ID: "usr_dup_author", Email: "dup@example.com", DisplayName: "Dup", CreatedAt: now}
	if err := st.CreateUser(ctx, author, []byte("hash"), sha256.Sum256([]byte(author.ID))); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateGroup(ctx, store.NewGroup{ID: "grp_dup", Visibility: "public", OwnerID: author.ID, Name: "Dup", InviteToken: "inv_dup"}, now, 0); err != nil {
		t.Fatal(err)
	}
	message := model.GroupMessage{ID: "msg_dup", GroupID: "grp_dup", Author: author, Body: "hello", CreatedAt: now}
	if err := st.AddMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	err = st.AddMessage(ctx, message)
	if err == nil || errors.Is(err, store.ErrNotFound) {
		t.Fatalf("duplicate id must be a conflict, not ErrNotFound: %v", err)
	}
}
