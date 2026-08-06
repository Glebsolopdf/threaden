package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"voice-rooms/internal/model"
)

func (s *Store) InsertRoom(ctx context.Context, code, ownerID string, now time.Time, ttl time.Duration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create room: %w", err)
	}
	defer tx.Rollback()
	expires := now.Add(ttl).Unix()
	if _, err = tx.ExecContext(ctx, `INSERT INTO rooms(code, owner_id, created_at, expires_at) VALUES(?, ?, ?, ?)`,
		code, ownerID, now.Unix(), expires); err != nil {
		if isConstraint(err) {
			return ErrConflict
		}
		return fmt.Errorf("insert room: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO room_members(room_code, user_id, joined_at) VALUES(?, ?, ?)`,
		code, ownerID, now.Unix()); err != nil {
		return fmt.Errorf("insert owner membership: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit room: %w", err)
	}
	return nil
}

func (s *Store) GetRoom(ctx context.Context, code, userID string, now time.Time, max int) (model.Room, error) {
	var room model.Room
	var created, expires int64
	var ownerCreated int64
	err := s.db.QueryRowContext(ctx, `
		SELECT r.code, r.created_at, r.expires_at,
		       u.id, u.display_name, u.avatar, u.created_at,
		       (SELECT COUNT(*) FROM room_members rm
		          JOIN users member ON member.id = rm.user_id
		         WHERE rm.room_code = r.code)
		FROM rooms r JOIN users u ON u.id = r.owner_id
		WHERE r.code = ? AND r.expires_at > ?
		  AND EXISTS (SELECT 1 FROM room_members rm WHERE rm.room_code = r.code AND rm.user_id = ?)`,
		code, now.Unix(), userID).Scan(
		&room.Code, &created, &expires, &room.Owner.ID, &room.Owner.DisplayName,
		&room.Owner.Avatar, &ownerCreated, &room.ParticipantCount)
	if err == sql.ErrNoRows {
		return model.Room{}, ErrNotFound
	}
	if err != nil {
		return model.Room{}, fmt.Errorf("get room: %w", err)
	}
	room.CreatedAt = time.Unix(created, 0).UTC()
	room.ExpiresAt = time.Unix(expires, 0).UTC()
	room.Owner.CreatedAt = time.Unix(ownerCreated, 0).UTC()
	room.MaxParticipants = max
	room.Members = make([]model.Member, 0, room.ParticipantCount)
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.display_name, u.avatar, rm.joined_at
		FROM room_members rm JOIN users u ON u.id = rm.user_id
		WHERE rm.room_code = ?
		ORDER BY rm.joined_at, u.id`, code)
	if err != nil {
		return model.Room{}, fmt.Errorf("list room members: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var member model.Member
		var joined int64
		if err := rows.Scan(&member.ID, &member.DisplayName, &member.Avatar, &joined); err != nil {
			return model.Room{}, fmt.Errorf("scan room member: %w", err)
		}
		member.JoinedAt = time.Unix(joined, 0).UTC()
		room.Members = append(room.Members, member)
	}
	if err := rows.Err(); err != nil {
		return model.Room{}, fmt.Errorf("iterate room members: %w", err)
	}
	return room, nil
}

func (s *Store) JoinRoom(ctx context.Context, code, userID string, now time.Time, max int) (model.Room, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Room{}, fmt.Errorf("begin join room: %w", err)
	}
	defer tx.Rollback()
	var expires int64
	err = tx.QueryRowContext(ctx, `
		SELECT r.expires_at FROM rooms r
		JOIN users owner ON owner.id = r.owner_id
		WHERE r.code = ? AND r.expires_at > ?`,
		code, now.Unix()).Scan(&expires)
	if err == sql.ErrNoRows {
		return model.Room{}, ErrNotFound
	}
	if err != nil {
		return model.Room{}, fmt.Errorf("find room for join: %w", err)
	}
	var memberExists int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM room_members WHERE room_code = ? AND user_id = ?`,
		code, userID).Scan(&memberExists); err != nil {
		return model.Room{}, fmt.Errorf("check membership: %w", err)
	}
	if memberExists == 0 {
		var count int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM room_members WHERE room_code = ?`, code).Scan(&count); err != nil {
			return model.Room{}, fmt.Errorf("count members: %w", err)
		}
		if count >= max {
			return model.Room{}, ErrRoomFull
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO room_members(room_code, user_id, joined_at) VALUES(?, ?, ?)`,
			code, userID, now.Unix()); err != nil {
			if isConstraint(err) {
				return model.Room{}, ErrConflict
			}
			return model.Room{}, fmt.Errorf("join room: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE rooms SET empty_since_at = NULL WHERE code = ?`, code); err != nil {
		return model.Room{}, fmt.Errorf("mark room occupied: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return model.Room{}, fmt.Errorf("commit join: %w", err)
	}
	return s.GetRoom(ctx, code, userID, now, max)
}

func (s *Store) LeaveRoom(ctx context.Context, code, userID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin leave room: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		DELETE FROM room_members
		WHERE room_code = ? AND user_id = ?
		  AND EXISTS (SELECT 1 FROM rooms WHERE code = ? AND expires_at > ?)`,
		code, userID, code, now.Unix())
	if err != nil {
		return fmt.Errorf("leave room: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rooms WHERE code = ? AND expires_at > ?`,
			code, now.Unix()).Scan(&exists); err != nil {
			return fmt.Errorf("check room: %w", err)
		}
		if exists == 0 {
			return ErrNotFound
		}
	}
	var members int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM room_members WHERE room_code = ?`, code).Scan(&members); err != nil {
		return fmt.Errorf("count room members after leave: %w", err)
	}
	if members == 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE rooms SET empty_since_at = ? WHERE code = ?`, now.Unix(), code); err != nil {
			return fmt.Errorf("mark room empty: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteRoom(ctx context.Context, code, userID string, now time.Time, terminate func() error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete room: %w", err)
	}
	defer tx.Rollback()
	var ownerID string
	err = tx.QueryRowContext(ctx, `SELECT owner_id FROM rooms WHERE code = ? AND expires_at > ?`,
		code, now.Unix()).Scan(&ownerID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("find room for delete: %w", err)
	}
	if ownerID != userID {
		return ErrForbidden
	}
	if err := terminate(); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM rooms WHERE code = ?`, code); err != nil {
		return fmt.Errorf("delete room: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit delete room: %w", err)
	}
	return nil
}

func (s *Store) ExpiredRoomCodes(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT code FROM rooms WHERE expires_at <= ?`, now.Unix())
	if err != nil {
		return nil, fmt.Errorf("list expired rooms: %w", err)
	}
	defer rows.Close()
	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan expired room: %w", err)
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

func (s *Store) EmptyRoomCodes(ctx context.Context, cutoff time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT code FROM rooms
		WHERE empty_since_at IS NOT NULL AND empty_since_at <= ?`, cutoff.Unix())
	if err != nil {
		return nil, fmt.Errorf("list empty rooms: %w", err)
	}
	defer rows.Close()
	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan empty room: %w", err)
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

func (s *Store) DeleteEmptyRoom(ctx context.Context, code string, cutoff time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM rooms WHERE code = ? AND empty_since_at IS NOT NULL AND empty_since_at <= ?`,
		code, cutoff.Unix())
	if err != nil {
		return fmt.Errorf("delete empty room: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredRoom(ctx context.Context, code string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM rooms WHERE code = ? AND expires_at <= ?`, code, now.Unix())
	if err != nil {
		return fmt.Errorf("delete expired room: %w", err)
	}
	return nil
}

func Is(err, target error) bool { return errors.Is(err, target) }
