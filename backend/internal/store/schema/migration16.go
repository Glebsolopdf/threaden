package schema

const migration16 = `
ALTER TABLE group_messages ADD COLUMN reply_snapshot TEXT;
`
