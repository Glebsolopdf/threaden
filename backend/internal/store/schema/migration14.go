package schema

const migration14 = `
CREATE TABLE IF NOT EXISTS group_message_reads (
	message_id TEXT NOT NULL REFERENCES group_messages(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	read_at INTEGER NOT NULL,
	PRIMARY KEY (message_id, user_id)
);

CREATE INDEX IF NOT EXISTS group_message_reads_user_idx ON group_message_reads(user_id);
`
