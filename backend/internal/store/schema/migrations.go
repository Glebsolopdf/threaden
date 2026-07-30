package schema

import "fmt"

const LatestVersion = 9

func Migration(version int) (string, error) {
	switch version {
	case 1:
		return migration1, nil
	case 2:
		return migration2, nil
	case 3:
		return migration3, nil
	case 4:
		return migration4, nil
	case 5:
		return migration5, nil
	case 6:
		return migration6, nil
	case 7:
		return migration7, nil
	case 8:
		return migration8, nil
	case 9:
		return migration9, nil
	default:
		return "", fmt.Errorf("unknown migration version %d", version)
	}
}

const migration1 = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	applied_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	display_name TEXT NOT NULL,
	avatar TEXT NOT NULL,
	password_hash BLOB NOT NULL,
	token_hash BLOB NOT NULL UNIQUE CHECK(length(token_hash) = 32),
	created_at INTEGER NOT NULL,
	last_seen_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS rooms (
	code TEXT PRIMARY KEY,
	owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS room_members (
	room_code TEXT NOT NULL REFERENCES rooms(code) ON DELETE CASCADE,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	joined_at INTEGER NOT NULL,
	PRIMARY KEY (room_code, user_id)
);

CREATE INDEX IF NOT EXISTS rooms_expires_at_idx ON rooms(expires_at);
CREATE INDEX IF NOT EXISTS rooms_owner_id_idx ON rooms(owner_id);
CREATE INDEX IF NOT EXISTS room_members_user_id_idx ON room_members(user_id);
`

const migration2 = `
CREATE TABLE users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	display_name TEXT NOT NULL,
	avatar TEXT NOT NULL,
	password_hash BLOB NOT NULL,
	token_hash BLOB NOT NULL UNIQUE CHECK(length(token_hash) = 32),
	created_at INTEGER NOT NULL
);

INSERT INTO users(id, email, display_name, avatar, password_hash, token_hash, created_at)
SELECT
	id,
	id || '@legacy.local',
	display_name,
	avatar,
	X'',
	token_hash,
	created_at
FROM temporary_users;

CREATE TABLE rooms_next (
	code TEXT PRIMARY KEY,
	owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);

INSERT INTO rooms_next(code, owner_id, created_at, expires_at)
SELECT code, owner_id, created_at, expires_at FROM rooms;

CREATE TABLE room_members_next (
	room_code TEXT NOT NULL REFERENCES rooms_next(code) ON DELETE CASCADE,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	joined_at INTEGER NOT NULL,
	PRIMARY KEY (room_code, user_id)
);

INSERT INTO room_members_next(room_code, user_id, joined_at)
SELECT room_code, user_id, joined_at FROM room_members;

DROP TABLE room_members;
DROP TABLE rooms;
DROP TABLE temporary_users;

ALTER TABLE rooms_next RENAME TO rooms;
ALTER TABLE room_members_next RENAME TO room_members;

CREATE INDEX rooms_expires_at_idx ON rooms(expires_at);
CREATE INDEX rooms_owner_id_idx ON rooms(owner_id);
CREATE INDEX room_members_user_id_idx ON room_members(user_id);
`

const migration3 = `
ALTER TABLE users ADD COLUMN last_seen_at INTEGER NOT NULL DEFAULT 0;

UPDATE users SET last_seen_at = created_at WHERE last_seen_at = 0;
`

const migration4 = `
CREATE TABLE IF NOT EXISTS groups (
	id TEXT PRIMARY KEY,
	visibility TEXT NOT NULL CHECK(visibility IN ('public', 'private')),
	owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	avatar TEXT NOT NULL,
	invite_token TEXT NOT NULL UNIQUE,
	created_at INTEGER NOT NULL,
	last_activity_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS group_members (
	group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	joined_at INTEGER NOT NULL,
	PRIMARY KEY (group_id, user_id)
);

CREATE TABLE IF NOT EXISTS group_messages (
	id TEXT PRIMARY KEY,
	group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
	author_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	body TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS group_voice_rooms (
	id TEXT PRIMARY KEY,
	group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS group_voice_members (
	user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
	voice_room_id TEXT NOT NULL REFERENCES group_voice_rooms(id) ON DELETE CASCADE,
	joined_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS groups_visibility_activity_idx ON groups(visibility, last_activity_at);
CREATE INDEX IF NOT EXISTS groups_owner_id_idx ON groups(owner_id);
CREATE INDEX IF NOT EXISTS group_members_user_id_idx ON group_members(user_id);
CREATE INDEX IF NOT EXISTS group_messages_group_created_idx ON group_messages(group_id, created_at);
CREATE INDEX IF NOT EXISTS group_voice_rooms_group_id_idx ON group_voice_rooms(group_id);
CREATE INDEX IF NOT EXISTS group_voice_members_voice_room_id_idx ON group_voice_members(voice_room_id);
`

const migration5 = `
CREATE INDEX IF NOT EXISTS group_members_group_joined_idx ON group_members(group_id, joined_at);
`

const migration6 = `
	UPDATE users SET avatar = '' WHERE avatar = '🙂';
	`

const migration7 = `
	ALTER TABLE groups ADD COLUMN scheduled_for_deletion_at INTEGER;
	ALTER TABLE groups ADD COLUMN protected_from_auto_delete INTEGER NOT NULL DEFAULT 0;

	CREATE TABLE IF NOT EXISTS rate_limit_buckets (
		key TEXT PRIMARY KEY,
		tokens REAL NOT NULL,
		updated_at INTEGER NOT NULL,
		expires_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS idempotency_keys (
		scope TEXT NOT NULL,
		user_id TEXT NOT NULL,
		key TEXT NOT NULL,
		response_id TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		expires_at INTEGER NOT NULL,
		PRIMARY KEY (scope, user_id, key)
	);

	CREATE INDEX IF NOT EXISTS groups_inactive_delete_idx
		ON groups(protected_from_auto_delete, scheduled_for_deletion_at, last_activity_at, created_at);
	CREATE INDEX IF NOT EXISTS rate_limit_buckets_expires_idx ON rate_limit_buckets(expires_at);
	CREATE INDEX IF NOT EXISTS idempotency_keys_expires_idx ON idempotency_keys(expires_at);
	`

const migration8 = `
	ALTER TABLE rooms ADD COLUMN empty_since_at INTEGER;
	CREATE INDEX IF NOT EXISTS rooms_empty_since_idx ON rooms(empty_since_at);
	`

const migration9 = `
	CREATE TABLE IF NOT EXISTS sessions (
		token_hash BLOB PRIMARY KEY CHECK(length(token_hash) = 32),
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at INTEGER NOT NULL,
		last_seen_at INTEGER NOT NULL,
		expires_at INTEGER NOT NULL
	);

	INSERT OR IGNORE INTO sessions(token_hash, user_id, created_at, last_seen_at, expires_at)
	SELECT token_hash, id, created_at, last_seen_at, last_seen_at + 604800
	FROM users
	WHERE length(token_hash) = 32;

	CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON sessions(expires_at);

	DELETE FROM rooms;
	UPDATE groups SET avatar = '👥' WHERE length(avatar) < 1 OR length(avatar) > 8;
	`
