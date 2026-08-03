package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"voice-rooms/internal/store/schema"
)

func Migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
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
