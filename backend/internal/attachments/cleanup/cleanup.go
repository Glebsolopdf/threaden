package cleanup

import (
	"context"
	"os"
	"path/filepath"
	"time"

	attachmentmeta "voice-rooms/internal/store/attachments"
)

type Service struct {
	DB        attachmentmeta.DB
	Root      string
	BatchSize int
}

func (s Service) RunOnce(ctx context.Context, now time.Time) error {
	items, err := attachmentmeta.DeleteExpired(ctx, s.DB, now, s.BatchSize)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := os.Remove(item.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := attachmentmeta.Delete(ctx, s.DB, item.ID); err != nil {
			return err
		}
	}
	return s.removeOrphans(ctx)
}

func (s Service) removeOrphans(ctx context.Context) error {
	entries, err := os.ReadDir(s.Root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(s.Root, entry.Name()))
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			path := filepath.Join(s.Root, entry.Name(), file.Name())
			exists, err := attachmentmeta.HasPath(ctx, s.DB, path)
			if err != nil {
				return err
			}
			if !exists {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					return err
				}
				continue
			}
			if _, err := os.Stat(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}
