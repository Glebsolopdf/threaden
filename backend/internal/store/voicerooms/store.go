package voicerooms

import (
	"context"
	"database/sql"
	"time"

	"voice-rooms/internal/model"
)

type VoiceRooms struct {
	db       *sql.DB
	notFound error
	roomFull error
}

func New(db *sql.DB, notFound, roomFull error) *VoiceRooms {
	return &VoiceRooms{db: db, notFound: notFound, roomFull: roomFull}
}

func (s *VoiceRooms) CreateVoiceRoom(ctx context.Context, id, groupID, name string, now time.Time, maxRooms int) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_voice_rooms WHERE group_id=?`, groupID).Scan(&count); e != nil {
		return e
	}
	if count >= maxRooms {
		return s.roomFull
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO group_voice_rooms(id,group_id,name,created_at) VALUES(?,?,?,?)`, id, groupID, name, now.Unix())
	if e != nil {
		return e
	}
	return tx.Commit()
}

func (s *VoiceRooms) VoiceRoom(ctx context.Context, id string) (model.GroupVoiceRoom, error) {
	var r model.GroupVoiceRoom
	var t int64
	e := s.db.QueryRowContext(ctx, `SELECT id,group_id,name,created_at,(SELECT COUNT(*) FROM group_voice_members vm WHERE vm.voice_room_id=group_voice_rooms.id) FROM group_voice_rooms WHERE id=?`, id).Scan(&r.ID, &r.GroupID, &r.Name, &t, &r.ParticipantCount)
	if e == sql.ErrNoRows {
		return r, s.notFound
	}
	r.CreatedAt = time.Unix(t, 0).UTC()
	return r, e
}

func (s *VoiceRooms) DeleteVoiceRoom(ctx context.Context, id string) error {
	result, e := s.db.ExecContext(ctx, `DELETE FROM group_voice_rooms WHERE id=?`, id)
	if e != nil {
		return e
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return s.notFound
	}
	return nil
}

func (s *VoiceRooms) EnterVoice(ctx context.Context, roomID, userID string, now time.Time) (string, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var old string
	_ = tx.QueryRowContext(ctx, `SELECT voice_room_id FROM group_voice_members WHERE user_id=?`, userID).Scan(&old)
	_, e = tx.ExecContext(ctx, `INSERT INTO group_voice_members(user_id,voice_room_id,joined_at) VALUES(?,?,?) ON CONFLICT(user_id) DO UPDATE SET voice_room_id=excluded.voice_room_id,joined_at=excluded.joined_at`, userID, roomID, now.Unix())
	if e != nil {
		return "", e
	}
	return old, tx.Commit()
}

func (s *VoiceRooms) LeaveVoice(ctx context.Context, roomID, userID string) error {
	_, e := s.db.ExecContext(ctx, `DELETE FROM group_voice_members WHERE voice_room_id=? AND user_id=?`, roomID, userID)
	return e
}
