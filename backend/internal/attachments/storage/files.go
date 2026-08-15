package storage

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"voice-rooms/internal/attachments"
	"voice-rooms/internal/model"
	attachmentmeta "voice-rooms/internal/store/attachments"
)

type DiskChecker interface {
	FreeBytes() (uint64, error)
}

type Input struct {
	Name string
	Mime string
	Size int64
	Open func() (io.ReadCloser, error)
}

type Service struct {
	Root      string
	Limits    attachments.Limits
	Processor attachments.Processor
	DB        attachmentmeta.DB
	Disk      DiskChecker
	mu        sync.Mutex
	reserved  int64
}

type Batch struct {
	service  *Service
	ownerID  string
	files    []attachments.ProcessedFile
	moved    []string
	reserved int64
	closed   bool
}

func (s *Service) Prepare(ctx context.Context, ownerID string, inputs []Input) (*Batch, error) {
	if len(inputs) == 0 || len(inputs) > s.Limits.MaxFilesPerMessage {
		return nil, attachments.ErrTooMany
	}
	reserve := int64(0)
	for _, input := range inputs {
		if input.Size <= 0 || input.Size > int64(s.Limits.MaxInputMediaBytes) && input.Size > int64(s.Limits.MaxArchiveBytes) {
			return nil, attachments.ErrTooLarge
		}
		reserve += int64(s.Limits.MaxArchiveBytes)
	}
	if err := s.reserve(ctx, ownerID, reserve); err != nil {
		return nil, err
	}
	batch := &Batch{service: s, ownerID: ownerID, reserved: reserve}
	for _, input := range inputs {
		reader, err := input.Open()
		if err != nil {
			batch.Rollback()
			return nil, fmt.Errorf("open attachment: %w", err)
		}
		processed, processErr := s.Processor.Process(ctx, reader, input.Name, input.Size, input.Mime)
		_ = reader.Close()
		if processErr != nil {
			batch.Rollback()
			return nil, processErr
		}
		batch.files = append(batch.files, processed)
	}
	return batch, nil
}

func (b *Batch) Commit(_ context.Context, messageID, groupID string, now time.Time) ([]model.Attachment, error) {
	if b.closed {
		return nil, attachments.ErrProcessing
	}
	if err := os.MkdirAll(b.service.Root, 0o700); err != nil {
		b.Rollback()
		return nil, err
	}
	items := make([]model.Attachment, 0, len(b.files))
	for _, file := range b.files {
		id, err := randomID()
		if err != nil {
			b.Rollback()
			return nil, err
		}
		dir := filepath.Join(b.service.Root, id[:2])
		if err := os.MkdirAll(dir, 0o700); err != nil {
			b.Rollback()
			return nil, err
		}
		destination := filepath.Join(dir, id)
		if err := moveFile(file.Path, destination); err != nil {
			b.Rollback()
			return nil, fmt.Errorf("store attachment: %w", err)
		}
		b.moved = append(b.moved, destination)
		items = append(items, model.Attachment{ID: id, MessageID: messageID, GroupID: groupID, OwnerID: b.ownerID, Kind: string(file.Kind), Mime: file.Mime, Name: file.DisplayName, Size: file.Size, Duration: file.Duration, Path: destination, CreatedAt: now, ExpiresAt: now.Add(b.service.Limits.Retention)})
	}
	b.release()
	b.closed = true
	return items, nil
}

func (b *Batch) Rollback() {
	for _, file := range b.files {
		_ = os.Remove(file.Path)
	}
	for _, file := range b.moved {
		_ = os.Remove(file)
	}
	if !b.closed {
		b.release()
		b.closed = true
	}
}

func (b *Batch) release() {
	if b.service == nil || b.reserved == 0 {
		return
	}
	b.service.mu.Lock()
	b.service.reserved -= b.reserved
	b.service.mu.Unlock()
	b.reserved = 0
}

func (s *Service) reserve(ctx context.Context, ownerID string, bytes int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := attachmentmeta.SumForOwner(ctx, s.DB, ownerID)
	if err != nil {
		return err
	}
	daily, err := attachmentmeta.SumCreatedSince(ctx, s.DB, ownerID, time.Now().Add(-24*time.Hour))
	if err != nil {
		return err
	}
	total, err := attachmentmeta.SumAll(ctx, s.DB)
	if err != nil {
		return err
	}
	createdAt, err := attachmentmeta.UserCreatedAt(ctx, s.DB, ownerID)
	if err != nil {
		return err
	}
	effective := attachments.EffectiveLimits(s.Limits, createdAt, time.Now())
	dailyExceeded := effective.MaxUserDailyBytes > 0 && uint64(daily+bytes) > effective.MaxUserDailyBytes
	if uint64(owner+s.reserved+bytes) > effective.MaxUserStoredBytes || dailyExceeded || uint64(total+s.reserved+bytes) > effective.MaxTotalBytes {
		return attachments.ErrQuotaExceeded
	}
	if s.Disk != nil {
		free, err := s.Disk.FreeBytes()
		if err != nil {
			return err
		}
		if free < s.Limits.MinFreeBytes || free-s.Limits.MinFreeBytes < uint64(bytes) {
			return attachments.ErrLowDisk
		}
	}
	s.reserved += bytes
	return nil
}

func randomID() (string, error) {
	data := make([]byte, 18)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return fmt.Sprintf("att_%x", data), nil
}

func moveFile(source, destination string) error {
	if err := os.Rename(source, destination); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		_ = os.Remove(destination)
		return err
	}
	if err := output.Close(); err != nil {
		_ = os.Remove(destination)
		return err
	}
	return os.Remove(source)
}
