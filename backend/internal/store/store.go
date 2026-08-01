package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"voice-rooms/internal/store/rate"
	"voice-rooms/internal/store/schema"

	_ "modernc.org/sqlite"
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
	db *sql.DB
	*rate.Store
}

func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := path
	if path == ":memory:" {
		dsn = "file:voice_rooms?mode=memory&cache=shared"
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	dsn += separator + "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// ponytail: one connection makes SQLite write ordering explicit; raise this only
	// after measured contention warrants per-connection PRAGMA management.
	db.SetMaxOpenConns(1)
	s := &Store{db: db, Store: rate.New(db)}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("inspect migrations: %w", err)
	}
	current := 0
	if exists == 0 {
		content, err := schema.Migration(1)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, content); err != nil {
			return fmt.Errorf("apply migration 1: %w", err)
		}
		content, err = schema.Migration(4)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, content); err != nil {
			return fmt.Errorf("apply migration 4: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(4, ?)`, time.Now().Unix()); err != nil {
			return fmt.Errorf("record migration 4: %w", err)
		}
		current = 4
	} else if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read migration version: %w", err)
	}
	if current == 1 {
		content, err := schema.Migration(2)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, content); err != nil {
			return fmt.Errorf("apply migration 2: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(2, ?)`, time.Now().Unix()); err != nil {
			return fmt.Errorf("record migration 2: %w", err)
		}
		current = 2
	}
	if current == 2 {
		content, err := schema.Migration(3)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, content); err != nil {
			return fmt.Errorf("apply migration 3: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(3, ?)`, time.Now().Unix()); err != nil {
			return fmt.Errorf("record migration 3: %w", err)
		}
		current = 3
	}
	if current == 3 {
		content, err := schema.Migration(4)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, content); err != nil {
			return fmt.Errorf("apply migration 4: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(4, ?)`, time.Now().Unix()); err != nil {
			return fmt.Errorf("record migration 4: %w", err)
		}
		current = 4
	}
	for current < schema.LatestVersion {
		next := current + 1
		content, err := schema.Migration(next)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, content); err != nil {
			return fmt.Errorf("apply migration %d: %w", next, err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`, next, time.Now().Unix()); err != nil {
			return fmt.Errorf("record migration %d: %w", next, err)
		}
		current = next
	}
	if current > schema.LatestVersion {
		return fmt.Errorf("database schema version %d is newer than this binary", current)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

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

func isConstraint(err error) bool {
	var sqliteErr *sqlite3.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code()&0xff == 19
}
