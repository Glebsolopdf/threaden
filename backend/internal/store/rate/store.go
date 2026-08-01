// Package rate persists token-bucket rate limits and IP abuse bans in SQLite.
package rate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) TakeRateLimit(
	ctx context.Context,
	key string,
	now time.Time,
	capacity int,
	refillPerSecond float64,
	ttl time.Duration,
) (bool, time.Duration, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, fmt.Errorf("begin rate limit: %w", err)
	}
	defer tx.Rollback()
	nowUnix := now.Unix()
	_, _ = tx.ExecContext(ctx, `DELETE FROM rate_limit_buckets WHERE expires_at <= ?`, nowUnix)
	var tokens float64
	var updated int64
	err = tx.QueryRowContext(ctx, `SELECT tokens, updated_at FROM rate_limit_buckets WHERE key = ?`, key).
		Scan(&tokens, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		tokens = float64(capacity)
		updated = nowUnix
	} else if err != nil {
		return false, 0, fmt.Errorf("read rate bucket: %w", err)
	}
	if elapsed := nowUnix - updated; elapsed > 0 {
		tokens = min(float64(capacity), tokens+float64(elapsed)*refillPerSecond)
	}
	if tokens < 1 {
		wait := time.Duration((1-tokens)/refillPerSecond) * time.Second
		if wait < time.Second {
			wait = time.Second
		}
		if err := upsertRateBucket(ctx, tx, key, tokens, nowUnix, now.Add(ttl).Unix()); err != nil {
			return false, 0, err
		}
		return false, wait, tx.Commit()
	}
	tokens--
	if err := upsertRateBucket(ctx, tx, key, tokens, nowUnix, now.Add(ttl).Unix()); err != nil {
		return false, 0, err
	}
	return true, 0, tx.Commit()
}

func upsertRateBucket(ctx context.Context, tx *sql.Tx, key string, tokens float64, updated, expires int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO rate_limit_buckets(key, tokens, updated_at, expires_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			tokens=excluded.tokens,
			updated_at=excluded.updated_at,
			expires_at=excluded.expires_at`,
		key, tokens, updated, expires)
	if err != nil {
		return fmt.Errorf("write rate bucket: %w", err)
	}
	return nil
}

// NoteViolation records one abusive request for the key. Once the key exceeds
// threshold violations inside window it becomes banned for a duration taken
// from steps, where the level is the number of bans recorded within
// escalationForget; repeat violations while banned are ignored. It returns the
// level of a newly created ban (0 when none) and when it expires.
func (s *Store) NoteViolation(
	ctx context.Context,
	key string,
	now time.Time,
	window time.Duration,
	threshold int,
	steps []time.Duration,
	escalationForget time.Duration,
) (int, time.Time, error) {
	if threshold <= 0 || len(steps) == 0 {
		return 0, time.Time{}, nil
	}
	nowUnix := now.Unix()
	windowSeconds := max(int64(1), int64(window/time.Second))
	forgetSeconds := max(int64(1), int64(escalationForget/time.Second))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("begin violation: %w", err)
	}
	defer tx.Rollback()
	_, _ = tx.ExecContext(ctx, `DELETE FROM ip_bans WHERE updated_at <= ?`, nowUnix-forgetSeconds)
	var violations, windowStart, until, updatedAt, banCount int64
	err = tx.QueryRowContext(ctx, `SELECT violations, window_start, until, updated_at, ban_count FROM ip_bans WHERE key = ?`, key).
		Scan(&violations, &windowStart, &until, &updatedAt, &banCount)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		violations, windowStart = 0, nowUnix
	case err != nil:
		return 0, time.Time{}, fmt.Errorf("read violation: %w", err)
	}
	if until > nowUnix {
		return 0, time.Time{}, nil
	}
	if nowUnix-windowStart > windowSeconds {
		violations, windowStart = 0, nowUnix
	}
	violations++
	level, untilAt := 0, int64(0)
	if violations >= int64(threshold) {
		if nowUnix-updatedAt <= forgetSeconds {
			banCount++
		} else {
			banCount = 1
		}
		level = int(min(banCount, int64(len(steps))))
		untilAt = nowUnix + int64(steps[level-1]/time.Second)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO ip_bans(key, violations, window_start, until, updated_at, ban_count)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			violations=excluded.violations,
			window_start=excluded.window_start,
			until=excluded.until,
			updated_at=excluded.updated_at,
			ban_count=excluded.ban_count`,
		key, violations, windowStart, untilAt, nowUnix, banCount)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("write violation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, time.Time{}, err
	}
	if level == 0 {
		return 0, time.Time{}, nil
	}
	return level, time.Unix(untilAt, 0).UTC(), nil
}

// RecordAccountBan records that a maximum-level ban was attributed to the
// account and returns how many such bans remain inside window, including the
// new one.
func (s *Store) RecordAccountBan(ctx context.Context, userID string, now time.Time, window time.Duration) (int, error) {
	cutoff := now.Add(-window).Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin account ban: %w", err)
	}
	defer tx.Rollback()
	_, _ = tx.ExecContext(ctx, `DELETE FROM account_bans WHERE created_at < ?`, cutoff)
	if _, err := tx.ExecContext(ctx, `INSERT INTO account_bans(user_id, created_at) VALUES(?, ?)`, userID, now.Unix()); err != nil {
		return 0, fmt.Errorf("insert account ban: %w", err)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_bans WHERE user_id = ? AND created_at >= ?`, userID, cutoff).Scan(&count); err != nil {
		return 0, fmt.Errorf("count account bans: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

// IPBanActive reports whether the key is currently banned and until when.
func (s *Store) IPBanActive(ctx context.Context, key string, now time.Time) (bool, time.Time, error) {
	var until int64
	err := s.db.QueryRowContext(ctx, `SELECT until FROM ip_bans WHERE key = ?`, key).Scan(&until)
	if errors.Is(err, sql.ErrNoRows) {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}
	if until <= now.Unix() {
		return false, time.Time{}, nil
	}
	return true, time.Unix(until, 0).UTC(), nil
}
