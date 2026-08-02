package readreceipts

import (
	"context"
	"database/sql"
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
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO group_message_reads(message_id,user_id,read_at) SELECT id,?,? FROM group_messages WHERE group_id=? AND created_at<=?`, userID, now, groupID, created); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT m.id,m.author_id FROM group_messages m JOIN group_message_reads r ON r.message_id=m.id WHERE m.group_id=? AND r.user_id=? AND m.author_id<>? AND m.created_at<=?`, groupID, userID, userID, created)
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
