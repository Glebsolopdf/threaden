package store_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"voice-rooms/internal/model"
	store "voice-rooms/internal/store"
	"voice-rooms/internal/store/schema"
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

func TestMigratesLegacyTemporaryUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL);
		INSERT INTO schema_migrations(version, applied_at) VALUES(1, 1);
		CREATE TABLE temporary_users (
			id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			avatar TEXT NOT NULL,
			token_hash BLOB NOT NULL UNIQUE CHECK(length(token_hash) = 32),
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		);
		CREATE INDEX temporary_users_expires_at_idx ON temporary_users(expires_at);
		CREATE TABLE rooms (
			code TEXT PRIMARY KEY,
			owner_id TEXT NOT NULL REFERENCES temporary_users(id) ON DELETE CASCADE,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		);
		CREATE INDEX rooms_expires_at_idx ON rooms(expires_at);
		CREATE INDEX rooms_owner_id_idx ON rooms(owner_id);
		CREATE TABLE room_members (
			room_code TEXT NOT NULL REFERENCES rooms(code) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES temporary_users(id) ON DELETE CASCADE,
			joined_at INTEGER NOT NULL,
			PRIMARY KEY (room_code, user_id)
		);
		CREATE INDEX room_members_user_id_idx ON room_members(user_id);
	`)
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatalf("seed legacy db: %v close: %v", err, closeErr)
	}
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	version, err := st.MigrationVersion()
	if err != nil || version != schema.LatestVersion {
		t.Fatalf("migration version=%d err=%v", version, err)
	}
}
