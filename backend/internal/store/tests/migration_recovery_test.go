package store_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	store "voice-rooms/internal/store"
	"voice-rooms/internal/store/schema"

	_ "modernc.org/sqlite"
)

func TestMigration12RecoversMissingIPBansTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-ip-bans.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL);
		INSERT INTO schema_migrations(version, applied_at) VALUES(11, 1);
		CREATE TABLE users (id TEXT PRIMARY KEY);
		CREATE TABLE group_members (group_id TEXT, user_id TEXT, joined_at INTEGER);
		CREATE TABLE group_messages (id TEXT PRIMARY KEY, group_id TEXT, author_id TEXT, body TEXT, created_at INTEGER);
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
