package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"voice-rooms/internal/model"
	"voice-rooms/internal/store/rate"
	"voice-rooms/internal/store/readreceipts"
	"voice-rooms/internal/store/voicerooms"
	"voice-rooms/internal/store/welcomecache"

	sqlite "voice-rooms/internal/store/sqlite"

	sqlite3 "modernc.org/sqlite"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
	ErrForbidden  = errors.New("forbidden")
	ErrRoomFull   = errors.New("room full")
	ErrGroupLimit = errors.New("group limit reached")
)

type Store struct {
	db           *sql.DB
	welcomeCache welcomecache.Cache
	*rate.Store
	*voicerooms.VoiceRooms
}

func Open(path string) (*Store, error) {
	db, err := sqlite.Open(path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, Store: rate.New(db), VoiceRooms: voicerooms.New(db, ErrNotFound, ErrRoomFull)}
	if err := sqlite.Migrate(context.Background(), db); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) WelcomeStats(ctx context.Context, userID string, now time.Time) (model.WelcomeStats, error) {
	var exists string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE id=?`, userID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.WelcomeStats{}, ErrNotFound
		}
		return model.WelcomeStats{}, fmt.Errorf("check welcome user: %w", err)
	}
	return s.welcomeCache.Get(now, func() (model.WelcomeStats, error) {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return model.WelcomeStats{}, fmt.Errorf("begin welcome stats: %w", err)
		}
		defer tx.Rollback()
		cutoff := now.Add(-24 * time.Hour).Unix()
		var stats model.WelcomeStats
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_messages WHERE created_at>=?`, cutoff).Scan(&stats.Messages); err != nil {
			return model.WelcomeStats{}, fmt.Errorf("count welcome messages: %w", err)
		}
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE created_at>=?`, cutoff).Scan(&stats.NewUsers); err != nil {
			return model.WelcomeStats{}, fmt.Errorf("count welcome users: %w", err)
		}
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE created_at>=?`, cutoff).Scan(&stats.NewGroups); err != nil {
			return model.WelcomeStats{}, fmt.Errorf("count welcome groups: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return model.WelcomeStats{}, fmt.Errorf("commit welcome stats: %w", err)
		}
		return stats, nil
	})
}

func (s *Store) MarkGroupMessagesRead(ctx context.Context, groupID, userID, messageID string, now time.Time) ([]readreceipts.Receipt, error) {
	receipts, err := readreceipts.Mark(ctx, s.db, groupID, userID, messageID, now.Unix())
	if errors.Is(err, readreceipts.ErrNotFound) {
		return nil, ErrNotFound
	}
	return receipts, err
}

func (s *Store) MigrationVersion() (int, error) {
	var version int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	return version, err
}

func (s *Store) TouchUser(ctx context.Context, userID string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET last_seen_at = ? WHERE id = ?`, now.Unix(), userID)
	return err
}

func (s *Store) CountUserGroups(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE owner_id=?`, userID).Scan(&n)
	return n, err
}

func (s *Store) DeleteInactiveUsers(ctx context.Context, cutoff time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE last_seen_at < ?`, cutoff.Unix())
	return err
}

func (s *Store) DeleteInactiveUsersBatch(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id IN (SELECT id FROM users WHERE last_seen_at < ? ORDER BY last_seen_at, created_at LIMIT ?)`, cutoff.Unix(), limit)
	if err != nil {
		return 0, fmt.Errorf("delete inactive users batch: %w", err)
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

func (s *Store) CleanupBannedUser(ctx context.Context, userID string, now time.Time, messageLimit int) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin banned-user cleanup: %w", err)
	}
	defer tx.Rollback()
	roomNames, err := collectIDs(tx.QueryContext(ctx, `SELECT room_code FROM room_members WHERE user_id = ?`, userID))
	if err != nil {
		return nil, fmt.Errorf("list temporary room memberships: %w", err)
	}
	groupVoiceNames, err := collectIDs(tx.QueryContext(ctx, `SELECT 'group:' || vr.group_id || ':' || vr.id FROM group_voice_members vm JOIN group_voice_rooms vr ON vr.id = vm.voice_room_id WHERE vm.user_id = ?`, userID))
	if err != nil {
		return nil, fmt.Errorf("list group voice memberships: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM room_members WHERE user_id = ?`, userID); err != nil {
		return nil, fmt.Errorf("delete temporary room memberships: %w", err)
	}
	if len(roomNames) > 0 {
		if _, err = tx.ExecContext(ctx, `UPDATE rooms SET empty_since_at = ? WHERE code IN (SELECT code FROM rooms WHERE code IN (`+placeholders(len(roomNames))+`) AND NOT EXISTS (SELECT 1 FROM room_members rm WHERE rm.room_code = rooms.code))`, prependArgs(now.Unix(), roomNames)...); err != nil {
			return nil, fmt.Errorf("mark temporary rooms empty: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM group_voice_members WHERE user_id = ?`, userID); err != nil {
		return nil, fmt.Errorf("delete group voice memberships: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM group_messages WHERE id IN (SELECT id FROM group_messages WHERE author_id = ? ORDER BY created_at DESC LIMIT ?)`, userID, messageLimit); err != nil {
		return nil, fmt.Errorf("delete recent user messages: %w", err)
	}
	roomNames = append(roomNames, groupVoiceNames...)
	return roomNames, tx.Commit()
}

func (s *Store) DeleteRecentMessagesByAuthor(ctx context.Context, groupID, userID string, since time.Time, limit int) (int, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM group_messages WHERE id IN (SELECT id FROM group_messages WHERE group_id = ? AND author_id = ? AND created_at >= ? ORDER BY created_at DESC LIMIT ?)`, groupID, userID, since.Unix(), limit)
	if err != nil {
		return 0, fmt.Errorf("delete recent messages by author: %w", err)
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

func (s *Store) DeleteRecentRepeatedMessages(ctx context.Context, groupID, userID, body string, since time.Time, limit int) (int, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM group_messages WHERE id IN (SELECT id FROM group_messages WHERE group_id = ? AND author_id = ? AND created_at >= ? AND lower(trim(body)) = ? ORDER BY created_at DESC LIMIT ?)`, groupID, userID, since.Unix(), body, limit)
	if err != nil {
		return 0, fmt.Errorf("delete repeated messages: %w", err)
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

func (s *Store) CreateGroupSpamWarning(ctx context.Context, groupID, reason string, now time.Time, messageCount, userCount int, cooldown, window time.Duration) (int, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, fmt.Errorf("begin group spam warning: %w", err)
	}
	defer tx.Rollback()
	_, _ = tx.ExecContext(ctx, `DELETE FROM group_spam_warnings WHERE created_at < ?`, now.Add(-window).Unix())
	var recent int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_spam_warnings WHERE group_id = ? AND created_at > ?`, groupID, now.Add(-cooldown).Unix()).Scan(&recent); err != nil {
		return 0, false, fmt.Errorf("count recent group spam warnings: %w", err)
	}
	if recent > 0 {
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_spam_warnings WHERE group_id = ? AND created_at >= ?`, groupID, now.Add(-window).Unix()).Scan(&recent); err != nil {
			return 0, false, fmt.Errorf("count group spam warnings: %w", err)
		}
		return recent, false, tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO group_spam_warnings(group_id,reason,message_count,user_count,created_at) VALUES(?,?,?,?,?)`, groupID, reason, messageCount, userCount, now.Unix()); err != nil {
		return 0, false, fmt.Errorf("insert group spam warning: %w", err)
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_spam_warnings WHERE group_id = ? AND created_at >= ?`, groupID, now.Add(-window).Unix()).Scan(&recent); err != nil {
		return 0, false, fmt.Errorf("count group spam warnings: %w", err)
	}
	return recent, true, tx.Commit()
}

func isConstraint(err error) bool {
	var sqliteErr *sqlite3.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code()&0xff == 19
}

func collectIDs(rows *sql.Rows, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}
	return scanIDs(rows)
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

func prependArgs(first any, rest []string) []any {
	args := make([]any, 0, len(rest)+1)
	args = append(args, first)
	for _, item := range rest {
		args = append(args, item)
	}
	return args
}
