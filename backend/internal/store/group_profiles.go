package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"voice-rooms/internal/model"
)

func (s *Store) GroupMembers(ctx context.Context, groupID, ownerID string) ([]model.GroupMember, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.id,u.display_name,u.avatar,gm.joined_at FROM group_members gm JOIN users u ON u.id=gm.user_id WHERE gm.group_id=? ORDER BY gm.joined_at,u.display_name`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []model.GroupMember{}
	for rows.Next() {
		var member model.GroupMember
		var joined int64
		if err := rows.Scan(&member.ID, &member.DisplayName, &member.Avatar, &joined); err != nil {
			return nil, err
		}
		member.JoinedAt = time.Unix(joined, 0).UTC()
		member.Role = "member"
		if member.ID == ownerID {
			member.Role = "owner"
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *Store) DeleteGroup(ctx context.Context, groupID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM groups WHERE id=?`, groupID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) LeaveGroup(ctx context.Context, groupID, userID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM group_members WHERE group_id=? AND user_id=?`, groupID, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RemoveGroupMember(ctx context.Context, groupID, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin remove group member: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id=? AND user_id=?`, groupID, userID)
	if err != nil {
		return fmt.Errorf("remove group member: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	_, err = tx.ExecContext(ctx, `
		DELETE FROM group_voice_members
		WHERE user_id = ?
		  AND voice_room_id IN (SELECT id FROM group_voice_rooms WHERE group_id = ?)`, userID, groupID)
	if err != nil {
		return fmt.Errorf("remove member from group voice rooms: %w", err)
	}
	return tx.Commit()
}

type InactiveGroupCandidate struct {
	ID                   string
	Reason               string
	MessageCount         int
	EstimatedStoredBytes int
	ScheduledForDeletion *time.Time
}

func (s *Store) InactiveGroupCandidates(ctx context.Context, cutoff time.Time, limit int) ([]InactiveGroupCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id,
		       CASE WHEN EXISTS(SELECT 1 FROM group_messages m WHERE m.group_id = g.id)
		            THEN 'last_message_older_than_retention'
		            ELSE 'empty_group_older_than_retention' END,
		       (SELECT COUNT(*) FROM group_messages m WHERE m.group_id = g.id),
		       length(g.avatar) + COALESCE((SELECT SUM(length(m.body)) FROM group_messages m WHERE m.group_id = g.id), 0),
		       g.scheduled_for_deletion_at
		FROM groups g
		WHERE g.protected_from_auto_delete = 0
		  AND g.last_activity_at < ?
		ORDER BY g.last_activity_at
		LIMIT ?`, cutoff.Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("list inactive groups: %w", err)
	}
	defer rows.Close()
	var out []InactiveGroupCandidate
	for rows.Next() {
		var item InactiveGroupCandidate
		var scheduled sql.NullInt64
		if err := rows.Scan(&item.ID, &item.Reason, &item.MessageCount, &item.EstimatedStoredBytes, &scheduled); err != nil {
			return nil, err
		}
		if scheduled.Valid {
			t := time.Unix(scheduled.Int64, 0).UTC()
			item.ScheduledForDeletion = &t
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ScheduleInactiveGroups(ctx context.Context, cutoff, scheduledAt time.Time, limit int) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE groups
		SET scheduled_for_deletion_at = ?
		WHERE id IN (
			SELECT id FROM groups
			WHERE protected_from_auto_delete = 0
			  AND scheduled_for_deletion_at IS NULL
			  AND last_activity_at < ?
			ORDER BY last_activity_at
			LIMIT ?
		)`, scheduledAt.Unix(), cutoff.Unix(), limit)
	if err != nil {
		return 0, fmt.Errorf("schedule inactive groups: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

func (s *Store) DeleteScheduledGroups(ctx context.Context, now time.Time, limit int) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin scheduled group delete: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM groups
		WHERE protected_from_auto_delete = 0
		  AND scheduled_for_deletion_at IS NOT NULL
		  AND scheduled_for_deletion_at <= ?
		ORDER BY scheduled_for_deletion_at
		LIMIT ?`, now.Unix(), limit)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id); err != nil {
			return nil, fmt.Errorf("delete scheduled group %s: %w", id, err)
		}
	}
	return ids, tx.Commit()
}

func (s *Store) DeleteInactiveGroupsNow(ctx context.Context, cutoff time.Time, limit int) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin inactive group delete: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM groups
		WHERE protected_from_auto_delete = 0
		  AND last_activity_at < ?
		ORDER BY last_activity_at
		LIMIT ?`, cutoff.Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("list inactive groups to delete: %w", err)
	}
	ids, err := scanIDs(rows)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id); err != nil {
			return nil, fmt.Errorf("delete inactive group %s: %w", id, err)
		}
	}
	return ids, tx.Commit()
}

func (s *Store) DeleteOldestMessages(ctx context.Context, olderThan time.Time, limit int) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM group_messages
		WHERE id IN (
			SELECT id FROM group_messages
			WHERE created_at < ?
			ORDER BY created_at
			LIMIT ?
		)`, olderThan.Unix(), limit)
	if err != nil {
		return 0, fmt.Errorf("delete oldest messages: %w", err)
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

func scanIDs(rows *sql.Rows) ([]string, error) {
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) ReserveIdempotencyKey(
	ctx context.Context,
	scope string,
	userID string,
	key string,
	responseID string,
	now time.Time,
	ttl time.Duration,
) (string, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("begin idempotency: %w", err)
	}
	defer tx.Rollback()
	_, _ = tx.ExecContext(ctx, `DELETE FROM idempotency_keys WHERE expires_at <= ?`, now.Unix())
	var existing string
	err = tx.QueryRowContext(ctx, `
		SELECT response_id FROM idempotency_keys
		WHERE scope = ? AND user_id = ? AND key = ?`, scope, userID, key).Scan(&existing)
	if err == nil {
		return existing, false, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("read idempotency key: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO idempotency_keys(scope, user_id, key, response_id, created_at, expires_at)
		VALUES(?, ?, ?, ?, ?, ?)`,
		scope, userID, key, responseID, now.Unix(), now.Add(ttl).Unix())
	if err != nil {
		return "", false, fmt.Errorf("insert idempotency key: %w", err)
	}
	return responseID, true, tx.Commit()
}
