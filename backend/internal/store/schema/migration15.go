package schema

const migration15 = `
ALTER TABLE group_messages ADD COLUMN reply_to_id TEXT REFERENCES group_messages(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS group_messages_reply_to_idx ON group_messages(reply_to_id);
`
