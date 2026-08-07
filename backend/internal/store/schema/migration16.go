package schema

const migration16 = `
ALTER TABLE users ADD COLUMN welcome_seen_at INTEGER NOT NULL DEFAULT 0;
`
