package schema

const migration17 = `
CREATE TABLE IF NOT EXISTS groups (
    id TEXT PRIMARY KEY,
    visibility TEXT NOT NULL DEFAULT 'public',
    owner_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    avatar TEXT NOT NULL DEFAULT '',
    invite_token TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT 0,
    last_activity_at INTEGER NOT NULL DEFAULT 0
);
ALTER TABLE groups ADD COLUMN isolated_until INTEGER NOT NULL DEFAULT 0;
ALTER TABLE groups ADD COLUMN isolation_level INTEGER NOT NULL DEFAULT 0;
ALTER TABLE groups ADD COLUMN isolation_raised_at INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS group_join_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip TEXT NOT NULL,
    joined_at INTEGER NOT NULL,
    successful INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS group_join_events_group_time_idx ON group_join_events(group_id, joined_at);
CREATE INDEX IF NOT EXISTS group_join_events_ip_time_idx ON group_join_events(ip, joined_at);

CREATE TABLE IF NOT EXISTS message_spam_cooldowns (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    level INTEGER NOT NULL DEFAULT 0,
    until INTEGER NOT NULL DEFAULT 0,
    last_violation INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (group_id, user_id)
);
`

const migration10 = `
	ALTER TABLE sessions ADD COLUMN reviewed_at INTEGER;
	UPDATE sessions SET reviewed_at = created_at WHERE reviewed_at IS NULL;
`

const migration11 = `
	CREATE TABLE IF NOT EXISTS ip_bans (
		key TEXT PRIMARY KEY,
		violations INTEGER NOT NULL,
		window_start INTEGER NOT NULL,
		until INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS ip_bans_until_idx ON ip_bans(until);
`

const migration18 = `
ALTER TABLE group_messages ADD COLUMN kind TEXT NOT NULL DEFAULT 'chat';
CREATE INDEX IF NOT EXISTS group_messages_kind_idx ON group_messages(group_id, kind, created_at);
`

const migration19 = `
ALTER TABLE group_messages ADD COLUMN created_at_nanos INTEGER NOT NULL DEFAULT 0;
ALTER TABLE group_members ADD COLUMN joined_at_nanos INTEGER NOT NULL DEFAULT 0;
UPDATE group_messages SET created_at_nanos = created_at * 1000000000 WHERE created_at_nanos = 0;
UPDATE group_members SET joined_at_nanos = joined_at * 1000000000 WHERE joined_at_nanos = 0;
CREATE INDEX IF NOT EXISTS group_messages_created_nanos_idx ON group_messages(group_id, created_at_nanos);
`

const migration20 = `
ALTER TABLE group_messages ADD COLUMN event TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS group_messages_event_idx ON group_messages(group_id, event, created_at);
`

const migration21 = `
CREATE TABLE IF NOT EXISTS attachments (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES group_messages(id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK(kind IN ('image','video','file','archive')),
    mime TEXT NOT NULL,
    name TEXT NOT NULL,
    size INTEGER NOT NULL CHECK(size > 0),
    path TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS attachments_message_idx ON attachments(message_id, created_at, id);
CREATE INDEX IF NOT EXISTS attachments_owner_created_idx ON attachments(owner_id, created_at);
CREATE INDEX IF NOT EXISTS attachments_expires_idx ON attachments(expires_at, id);
CREATE INDEX IF NOT EXISTS attachments_group_idx ON attachments(group_id);
`

const migration22 = `
CREATE TABLE IF NOT EXISTS attachment_delete_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    execute_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS attachment_delete_requests_user_idx
    ON attachment_delete_requests(user_id);
CREATE INDEX IF NOT EXISTS attachment_delete_requests_execute_idx
    ON attachment_delete_requests(execute_at, id);
`

const migration23 = `
CREATE TABLE IF NOT EXISTS account_blocks (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    until INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS account_blocks_until_idx ON account_blocks(until);
DELETE FROM ip_bans;
`

const migration24 = `
DROP INDEX IF EXISTS attachments_message_idx;
DROP INDEX IF EXISTS attachments_owner_created_idx;
DROP INDEX IF EXISTS attachments_expires_idx;
DROP INDEX IF EXISTS attachments_group_idx;
ALTER TABLE attachments RENAME TO attachments_legacy;
CREATE TABLE attachments (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES group_messages(id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK(kind IN ('image','video','audio','file','archive')),
    mime TEXT NOT NULL,
    name TEXT NOT NULL,
    size INTEGER NOT NULL CHECK(size > 0),
    path TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
INSERT INTO attachments(id,message_id,group_id,owner_id,kind,mime,name,size,path,created_at,expires_at)
SELECT id,message_id,group_id,owner_id,kind,mime,name,size,path,created_at,expires_at FROM attachments_legacy;
DROP TABLE attachments_legacy;
CREATE INDEX attachments_message_idx ON attachments(message_id, created_at, id);
CREATE INDEX attachments_owner_created_idx ON attachments(owner_id, created_at);
CREATE INDEX attachments_expires_idx ON attachments(expires_at, id);
CREATE INDEX attachments_group_idx ON attachments(group_id);
`
