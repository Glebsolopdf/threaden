package groupmessages

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"voice-rooms/internal/model"
	attachmentstore "voice-rooms/internal/store/attachments"
)

type DB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const selectMessage = `
	SELECT m.id,m.group_id,m.kind,m.event,m.body,m.created_at,m.created_at_nanos,u.id,u.display_name,u.avatar,u.created_at,
		rm.id,rm.kind,rm.event,rm.body,ru.id,ru.display_name,ru.avatar,ru.created_at
	FROM group_messages m
	JOIN users u ON u.id=m.author_id
	LEFT JOIN group_messages rm ON rm.id=m.reply_to_id
	LEFT JOIN users ru ON ru.id=rm.author_id`

func scan(row interface{ Scan(...any) error }) (model.GroupMessage, error) {
	var m model.GroupMessage
	var created, createdNanos, authorCreated int64
	var replyID, replyKind, replyEvent, replyBody, replyAuthorID, replyName, replyAvatar sql.NullString
	var replyCreated sql.NullInt64
	err := row.Scan(&m.ID, &m.GroupID, &m.Kind, &m.Event, &m.Body, &created, &createdNanos, &m.Author.ID, &m.Author.DisplayName, &m.Author.Avatar, &authorCreated,
		&replyID, &replyKind, &replyEvent, &replyBody, &replyAuthorID, &replyName, &replyAvatar, &replyCreated)
	if err == sql.ErrNoRows {
		return m, sql.ErrNoRows
	}
	if err != nil {
		return m, err
	}
	if m.Kind != "system" {
		m.Kind = "chat"
	}
	if createdNanos > 0 {
		m.CreatedAt = time.Unix(0, createdNanos).UTC()
	} else {
		m.CreatedAt = time.Unix(created, 0).UTC()
	}
	m.Author.CreatedAt = time.Unix(authorCreated, 0).UTC()
	if replyID.Valid {
		m.ReplyTo = &model.MessageReference{ID: replyID.String, Kind: replyKind.String, Event: replyEvent.String, Body: replyBody.String, Author: model.User{ID: replyAuthorID.String, DisplayName: replyName.String, Avatar: replyAvatar.String}}
		if replyCreated.Valid {
			m.ReplyTo.Author.CreatedAt = time.Unix(replyCreated.Int64, 0).UTC()
		}
	}
	return m, nil
}

func loadAttachments(ctx context.Context, db DB, messageID string) ([]model.Attachment, error) {
	return attachmentstore.ListForMessage(ctx, db, messageID)
}

func List(ctx context.Context, db DB, groupID string, cutoff time.Time, limit int, reader string) ([]model.GroupMessage, error) {
	rows, err := db.QueryContext(ctx, selectMessage+` WHERE m.group_id=? AND COALESCE(m.created_at_nanos,m.created_at*1000000000)>=? ORDER BY m.created_at_nanos DESC, m.created_at DESC LIMIT ?`, groupID, cutoff.UnixNano(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.GroupMessage{}
	for rows.Next() {
		m, scanErr := scan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Attachments, err = loadAttachments(ctx, db, out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
		out[left], out[right] = out[right], out[left]
	}
	return out, nil
}

func Get(ctx context.Context, db DB, id string) (model.GroupMessage, error) {
	m, err := scan(db.QueryRowContext(ctx, selectMessage+` WHERE m.id=?`, id))
	if err != nil {
		return m, err
	}
	m.Attachments, err = loadAttachments(ctx, db, m.ID)
	return m, err
}

func Add(ctx context.Context, db DB, m model.GroupMessage) error {
	var replyID any
	if m.ReplyTo != nil {
		replyID = m.ReplyTo.ID
	}
	kind := m.Kind
	if kind != "system" {
		kind = "chat"
	}
	_, err := db.ExecContext(ctx, `INSERT INTO group_messages(id,group_id,author_id,body,created_at,reply_to_id,kind,event,created_at_nanos) VALUES(?,?,?,?,?,?,?,?,?)`, m.ID, m.GroupID, m.Author.ID, m.Body, m.CreatedAt.Unix(), replyID, kind, m.Event, m.CreatedAt.UnixNano())
	return err
}

func Delete(ctx context.Context, db DB, id string) (string, error) {
	var groupID string
	err := db.QueryRowContext(ctx, `SELECT group_id FROM group_messages WHERE id=?`, id).Scan(&groupID)
	if err != nil {
		return "", err
	}
	if _, err = db.ExecContext(ctx, `DELETE FROM group_messages WHERE id=?`, id); err != nil {
		return "", fmt.Errorf("delete message: %w", err)
	}
	return groupID, nil
}
