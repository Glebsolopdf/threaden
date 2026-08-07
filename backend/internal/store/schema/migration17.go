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
