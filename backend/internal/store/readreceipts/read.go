package readreceipts

import (
	"context"
	"database/sql"
	"strings"
)

type Receipt struct{ MessageID, AuthorID string }

func Mark(ctx context.Context, db *sql.DB, groupID, userID, messageID string, now int64) ([]Receipt, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var created int64
	if err = tx.QueryRowContext(ctx, `SELECT created_at FROM group_messages WHERE id=? AND group_id=?`, messageID, groupID).Scan(&created); err == sql.ErrNoRows {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `INSERT OR IGNORE INTO group_message_reads(message_id,user_id,read_at) SELECT id,?,? FROM group_messages WHERE group_id=? AND created_at<=? RETURNING message_id`, userID, now, groupID, created)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, tx.Commit()
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, userID)
	rows, err = tx.QueryContext(ctx, `SELECT m.id,m.author_id FROM group_messages m WHERE m.id IN (`+placeholders+`) AND m.author_id<>?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var receipts []Receipt
	for rows.Next() {
		var receipt Receipt
		if err = rows.Scan(&receipt.MessageID, &receipt.AuthorID); err != nil {
			return nil, err
		}
		receipts = append(receipts, receipt)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return receipts, nil
}
