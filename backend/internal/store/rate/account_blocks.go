package rate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) SetAccountBlock(ctx context.Context, userID string, until time.Time) error {
	if userID == "" || !until.After(time.Time{}) {
		return fmt.Errorf("invalid account block")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO account_blocks(user_id, until) VALUES(?, ?)
		ON CONFLICT(user_id) DO UPDATE SET until=excluded.until`, userID, until.Unix())
	if err != nil {
		return fmt.Errorf("set account block: %w", err)
	}
	return nil
}

func (s *Store) AccountBlockActive(ctx context.Context, userID string, now time.Time) (bool, time.Time, error) {
	if userID == "" {
		return false, time.Time{}, nil
	}
	var until int64
	err := s.db.QueryRowContext(ctx, `SELECT until FROM account_blocks WHERE user_id=?`, userID).Scan(&until)
	if errors.Is(err, sql.ErrNoRows) {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, fmt.Errorf("read account block: %w", err)
	}
	if until <= now.Unix() {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM account_blocks WHERE user_id=? AND until<=?`, userID, now.Unix()); err != nil {
			return false, time.Time{}, fmt.Errorf("expire account block: %w", err)
		}
		return false, time.Time{}, nil
	}
	return true, time.Unix(until, 0).UTC(), nil
}
