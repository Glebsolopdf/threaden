package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"voice-rooms/internal/model"
)

const joinAnalysisWindow = 5 * time.Minute

var isolationDurations = []time.Duration{
	5 * time.Minute, 10 * time.Minute, time.Hour, 5 * time.Hour, 24 * time.Hour, 3 * 24 * time.Hour,
}

func (s *Store) RecordJoinEvent(ctx context.Context, groupID, userID, ip string, successful bool, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO group_join_events(group_id,user_id,ip,joined_at,successful) VALUES(?,?,?,?,?)`,
		groupID, userID, ip, now.Unix(), successful)
	if err != nil {
		return fmt.Errorf("record group join: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM group_join_events WHERE joined_at < ?`, now.Add(-24*time.Hour).Unix())
	return nil
}

func (s *Store) JoinAttackCandidates(ctx context.Context, groupID string, now time.Time) ([]string, int, error) {
	var members int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_members WHERE group_id=?`, groupID).Scan(&members); err != nil {
		return nil, 0, err
	}
	threshold := max(5, (members+4)/5)
	cutoff := now.Add(-joinAnalysisWindow).Unix()
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT e.user_id
		FROM group_join_events e JOIN users u ON u.id=e.user_id
		WHERE e.group_id=? AND e.successful=1 AND e.joined_at>=? AND u.created_at>=?
		AND e.ip IN (
			SELECT ip FROM group_join_events j JOIN users nu ON nu.id=j.user_id
			WHERE j.group_id=? AND j.successful=1 AND j.joined_at>=? AND nu.created_at>=?
			GROUP BY ip HAVING COUNT(DISTINCT j.user_id)>=?
		)`, groupID, cutoff, now.Add(-24*time.Hour).Unix(), groupID, cutoff, now.Add(-24*time.Hour).Unix(), threshold)
	if err != nil {
		return nil, threshold, fmt.Errorf("find join attack: %w", err)
	}
	defer rows.Close()
	var candidates []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, threshold, err
		}
		candidates = append(candidates, id)
	}
	return candidates, threshold, rows.Err()
}

func (s *Store) IsolateGroup(ctx context.Context, groupID string, candidates []string, now time.Time) (model.Group, []string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Group{}, nil, fmt.Errorf("begin group isolation: %w", err)
	}
	defer tx.Rollback()
	var level int
	var raised int64
	if err = tx.QueryRowContext(ctx, `SELECT isolation_level,isolation_raised_at FROM groups WHERE id=?`, groupID).Scan(&level, &raised); err != nil {
		if err == sql.ErrNoRows {
			return model.Group{}, nil, ErrNotFound
		}
		return model.Group{}, nil, err
	}
	if now.Unix()-raised > int64(joinAnalysisWindow/time.Second) {
		level = 0
	}
	level = min(level+1, len(isolationDurations))
	until := now.Add(isolationDurations[level-1]).Unix()
	if _, err = tx.ExecContext(ctx, `UPDATE groups SET isolated_until=?,isolation_level=?,isolation_raised_at=? WHERE id=?`, until, level, now.Unix(), groupID); err != nil {
		return model.Group{}, nil, err
	}
	removed := removeCandidates(ctx, tx, groupID, candidates)
	if err = tx.Commit(); err != nil {
		return model.Group{}, nil, err
	}
	g, err := s.Group(ctx, groupID)
	return g, removed, err
}

func removeCandidates(ctx context.Context, tx *sql.Tx, groupID string, candidates []string) []string {
	removed := make([]string, 0, len(candidates))
	for _, id := range candidates {
		result, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id=? AND user_id=? AND user_id<>(SELECT owner_id FROM groups WHERE id=?) AND user_id IN (SELECT id FROM users WHERE created_at>=?)`, groupID, id, groupID, time.Now().Add(-24*time.Hour).Unix())
		if err != nil {
			continue
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			continue
		}
		_, _ = tx.ExecContext(ctx, `DELETE FROM group_voice_members WHERE user_id=? AND voice_room_id IN (SELECT id FROM group_voice_rooms WHERE group_id=?)`, id, groupID)
		removed = append(removed, id)
	}
	return removed
}

func (s *Store) MessageCooldown(ctx context.Context, groupID, userID string, now time.Time) (time.Time, error) {
	var until int64
	err := s.db.QueryRowContext(ctx, `SELECT until FROM message_spam_cooldowns WHERE group_id=? AND user_id=?`, groupID, userID).Scan(&until)
	if err == sql.ErrNoRows || until <= now.Unix() {
		return time.Time{}, nil
	}
	return time.Unix(until, 0).UTC(), err
}

func (s *Store) RecordMessageViolation(ctx context.Context, groupID, userID string, now time.Time, durations []time.Duration) (time.Time, error) {
	if len(durations) == 0 {
		return time.Time{}, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback()
	var level int
	var last int64
	err = tx.QueryRowContext(ctx, `SELECT level,last_violation FROM message_spam_cooldowns WHERE group_id=? AND user_id=?`, groupID, userID).Scan(&level, &last)
	if err == sql.ErrNoRows || now.Unix()-last > int64(24*time.Hour/time.Second) {
		level = 0
	} else if err != nil {
		return time.Time{}, err
	}
	level = min(level+1, len(durations))
	until := now.Add(durations[level-1])
	_, err = tx.ExecContext(ctx, `INSERT INTO message_spam_cooldowns(group_id,user_id,level,until,last_violation) VALUES(?,?,?,?,?) ON CONFLICT(group_id,user_id) DO UPDATE SET level=excluded.level,until=excluded.until,last_violation=excluded.last_violation`, groupID, userID, level, until.Unix(), now.Unix())
	if err != nil {
		return time.Time{}, err
	}
	return until, tx.Commit()
}
