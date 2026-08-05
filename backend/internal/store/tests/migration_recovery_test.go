package store_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	store "voice-rooms/internal/store"
	"voice-rooms/internal/store/schema"

	_ "modernc.org/sqlite"
)

func TestRecoversMissingIPBansTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-ip-bans.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL);
		INSERT INTO schema_migrations(version, applied_at) VALUES(11, 1);
		CREATE TABLE users (id TEXT PRIMARY KEY);
		CREATE TABLE group_messages (id TEXT PRIMARY KEY, group_id TEXT NOT NULL, author_id TEXT NOT NULL, body TEXT NOT NULL, created_at INTEGER NOT NULL);
	`)
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatalf("seed incomplete v11 database: %v close: %v", err, closeErr)
	}

	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("recover migration: %v", err)
	}
	defer st.Close()
	version, err := st.MigrationVersion()
	if err != nil || version != schema.LatestVersion {
		t.Fatalf("unexpected recovered version %d: %v", version, err)
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
