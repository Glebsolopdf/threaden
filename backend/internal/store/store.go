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

	"voice-rooms/internal/store/schema"

	_ "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrForbidden = errors.New("forbidden")
	ErrRoomFull  = errors.New("room full")
)

type Store struct {
	db *sql.DB
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
	s := &Store{db: db}
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

func isConstraint(err error) bool {
	var sqliteErr *sqlite3.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code()&0xff == 19
}
