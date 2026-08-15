package attachments

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"voice-rooms/internal/model"
)

var ErrDeleteRequestNotFound = errors.New("attachment delete request not found")

func CreateDeleteRequest(ctx context.Context, db DB, userID string, createdAt, executeAt time.Time) (model.AttachmentDeleteRequest, error) {
	id, err := deleteRequestID()
	if err != nil {
		return model.AttachmentDeleteRequest{}, err
	}
	_, err = db.ExecContext(ctx, `INSERT OR IGNORE INTO attachment_delete_requests(id,user_id,created_at,execute_at) VALUES(?,?,?,?)`, id, userID, createdAt.Unix(), executeAt.Unix())
	if err != nil {
		return model.AttachmentDeleteRequest{}, fmt.Errorf("create attachment delete request: %w", err)
	}
	return GetDeleteRequest(ctx, db, userID)
}

func GetDeleteRequest(ctx context.Context, db DB, userID string) (model.AttachmentDeleteRequest, error) {
	var item model.AttachmentDeleteRequest
	var created, execute int64
	err := db.QueryRowContext(ctx, `SELECT id,created_at,execute_at FROM attachment_delete_requests WHERE user_id=?`, userID).Scan(&item.ID, &created, &execute)
	if err == sql.ErrNoRows {
		return item, ErrDeleteRequestNotFound
	}
	if err != nil {
		return item, err
	}
	item.UserID = userID
	item.CreatedAt = time.Unix(created, 0).UTC()
	item.ExecuteAt = time.Unix(execute, 0).UTC()
	return item, nil
}

func CancelDeleteRequest(ctx context.Context, db DB, userID string) error {
	result, err := db.ExecContext(ctx, `DELETE FROM attachment_delete_requests WHERE user_id=?`, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrDeleteRequestNotFound
	}
	return nil
}

func ListDueDeleteRequests(ctx context.Context, db DB, now time.Time, limit int) ([]model.AttachmentDeleteRequest, error) {
	if limit < 1 {
		limit = 100
	}
	rows, err := db.QueryContext(ctx, `SELECT id,user_id,created_at,execute_at FROM attachment_delete_requests WHERE execute_at<=? ORDER BY execute_at,id LIMIT ?`, now.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.AttachmentDeleteRequest{}
	for rows.Next() {
		var item model.AttachmentDeleteRequest
		var created, execute int64
		if err := rows.Scan(&item.ID, &item.UserID, &created, &execute); err != nil {
			return nil, err
		}
		item.CreatedAt = time.Unix(created, 0).UTC()
		item.ExecuteAt = time.Unix(execute, 0).UTC()
		items = append(items, item)
	}
	return items, rows.Err()
}

func DeleteDeleteRequest(ctx context.Context, db DB, id string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM attachment_delete_requests WHERE id=?`, id)
	return err
}

func ListUserAttachmentFiles(ctx context.Context, db DB, userID string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT path FROM attachments WHERE owner_id=?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := []string{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

func DeleteUserAttachmentMessages(ctx context.Context, db DB, userID string) ([]model.Attachment, error) {
	database, ok := db.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	})
	if !ok {
		return nil, errors.New("attachment delete requires a transactional database")
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,message_id,group_id,path FROM attachments WHERE owner_id=?`, userID)
	if err != nil {
		return nil, err
	}
	items := []model.Attachment{}
	for rows.Next() {
		var item model.Attachment
		if err := rows.Scan(&item.ID, &item.MessageID, &item.GroupID, &item.Path); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_messages WHERE id IN (SELECT DISTINCT message_id FROM attachments WHERE owner_id=?)`, userID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func deleteRequestID() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate attachment delete request id: %w", err)
	}
	return "adr_" + hex.EncodeToString(b), nil
}
