package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"voice-rooms/internal/store/schema"
)

func TestMigrateFreshDatabase(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate fresh: %v", err)
	}
	version := currentVersion(t, db)
	if version != schema.LatestVersion {
		t.Fatalf("fresh database should reach %d, got %d", schema.LatestVersion, version)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("second migrate must be idempotent: %v", err)
	}
	if version = currentVersion(t, db); version != schema.LatestVersion {
		t.Fatalf("idempotent migrate changed version to %d", version)
	}
}

func TestMigrateStepsThroughLegacyVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	legacy := `
CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL);
CREATE TABLE temporary_users (id TEXT PRIMARY KEY, display_name TEXT NOT NULL, avatar TEXT NOT NULL, token_hash BLOB NOT NULL, created_at INTEGER NOT NULL);
CREATE TABLE rooms (code TEXT PRIMARY KEY, owner_id TEXT NOT NULL, created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL);
CREATE TABLE room_members (room_code TEXT NOT NULL REFERENCES rooms(code) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, joined_at INTEGER NOT NULL, PRIMARY KEY (room_code, user_id));
INSERT INTO temporary_users(id, display_name, avatar, token_hash, created_at) VALUES('u1', 'Legacy', '👤', zeroblob(32), 0);
INSERT INTO schema_migrations(version, applied_at) VALUES(1, 0);
`
	if _, err := db.ExecContext(context.Background(), legacy); err != nil {
		t.Fatalf("build legacy v1 database: %v", err)
	}
	db.Close()

	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate legacy: %v", err)
	}
	if version := currentVersion(t, db); version != schema.LatestVersion {
		t.Fatalf("legacy database should reach %d, got %d", schema.LatestVersion, version)
	}
	var users int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 1 {
		t.Fatalf("legacy user should be carried over, got %d", users)
	}
}

func currentVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var version int
	if err := db.QueryRowContext(context.Background(), `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}
