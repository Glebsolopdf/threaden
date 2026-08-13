package attachments

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"voice-rooms/internal/model"
)

type DB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func Add(ctx context.Context, db DB, item model.Attachment) error {
	_, err := db.ExecContext(ctx, `INSERT INTO attachments(id,message_id,group_id,owner_id,kind,mime,name,size,path,created_at,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		item.ID, item.MessageID, item.GroupID, item.OwnerID, item.Kind, item.Mime, item.Name, item.Size, item.Path, item.CreatedAt.Unix(), item.ExpiresAt.Unix())
	if err != nil {
		return fmt.Errorf("insert attachment: %w", err)
	}
	return nil
}

func ListForMessage(ctx context.Context, db DB, messageID string) ([]model.Attachment, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,message_id,group_id,owner_id,kind,mime,name,size,path,created_at,expires_at FROM attachments WHERE message_id=? ORDER BY created_at,id`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.Attachment{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func Get(ctx context.Context, db DB, id string) (model.Attachment, error) {
	return scan(db.QueryRowContext(ctx, `SELECT id,message_id,group_id,owner_id,kind,mime,name,size,path,created_at,expires_at FROM attachments WHERE id=?`, id))
}

func DeleteExpired(ctx context.Context, db DB, now time.Time, limit int) ([]model.Attachment, error) {
	if limit < 1 {
		limit = 500
	}
	rows, err := db.QueryContext(ctx, `SELECT id,message_id,group_id,owner_id,kind,mime,name,size,path,created_at,expires_at FROM attachments WHERE expires_at<=? ORDER BY expires_at,id LIMIT ?`, now.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.Attachment{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func Delete(ctx context.Context, db DB, id string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM attachments WHERE id=?`, id)
	return err
}

func SumForOwner(ctx context.Context, db DB, ownerID string) (int64, error) {
	var total int64
	err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(size),0) FROM attachments WHERE owner_id=?`, ownerID).Scan(&total)
	return total, err
}

func SumCreatedSince(ctx context.Context, db DB, ownerID string, since time.Time) (int64, error) {
	var total int64
	err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(size),0) FROM attachments WHERE owner_id=? AND created_at>=?`, ownerID, since.Unix()).Scan(&total)
	return total, err
}

func SumAll(ctx context.Context, db DB) (int64, error) {
	var total int64
	err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(size),0) FROM attachments`).Scan(&total)
	return total, err
}

func HasPath(ctx context.Context, db DB, path string) (bool, error) {
	var value int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM attachments WHERE path=? LIMIT 1`, path).Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func scan(row interface{ Scan(...any) error }) (model.Attachment, error) {
	var item model.Attachment
	var created, expires int64
	if err := row.Scan(&item.ID, &item.MessageID, &item.GroupID, &item.OwnerID, &item.Kind, &item.Mime, &item.Name, &item.Size, &item.Path, &created, &expires); err != nil {
		return item, err
	}
	item.CreatedAt = time.Unix(created, 0).UTC()
	item.ExpiresAt = time.Unix(expires, 0).UTC()
	return item, nil
}
