package account

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"voice-rooms/internal/attachments"
	"voice-rooms/internal/model"
	attachmentstore "voice-rooms/internal/store/attachments"
)

const deleteGracePeriod = 5 * time.Minute

var ErrDeleteRequestNotFound = errors.New("attachment delete request not found")

type Service struct {
	DB        attachmentstore.DB
	Limits    attachments.Limits
	Root      string
	Logger    *slog.Logger
	Publisher MessageDeletionPublisher
}

type MessageDeletionPublisher interface {
	PublishMessageDeleted(context.Context, string, string) error
}

type QuotaSnapshot struct {
	StoredBytes    int64
	DailyBytes     int64
	MaxInputMedia  uint64
	MaxArchive     uint64
	MaxOutputMedia uint64
	MaxFiles       int
	MaxUserStored  uint64
	MaxUserDaily   uint64
	MaxTotal       uint64
	MinFree        uint64
	Retention      time.Duration
	PendingDelete  *model.AttachmentDeleteRequest
}

func (s Service) Quotas(ctx context.Context, userID string) (QuotaSnapshot, error) {
	stored, err := attachmentstore.SumForOwner(ctx, s.DB, userID)
	if err != nil {
		return QuotaSnapshot{}, fmt.Errorf("read stored attachment quota: %w", err)
	}
	daily, err := attachmentstore.SumCreatedSince(ctx, s.DB, userID, time.Now().Add(-24*time.Hour))
	if err != nil {
		return QuotaSnapshot{}, fmt.Errorf("read daily attachment quota: %w", err)
	}
	createdAt, err := attachmentstore.UserCreatedAt(ctx, s.DB, userID)
	if err != nil {
		return QuotaSnapshot{}, fmt.Errorf("read account age: %w", err)
	}
	effective := attachments.EffectiveLimits(s.Limits, createdAt, time.Now())
	pending, err := attachmentstore.GetDeleteRequest(ctx, s.DB, userID)
	if errors.Is(err, attachmentstore.ErrDeleteRequestNotFound) {
		err = nil
	}
	if err != nil {
		return QuotaSnapshot{}, fmt.Errorf("read attachment deletion state: %w", err)
	}
	var pendingPtr *model.AttachmentDeleteRequest
	if pending.ID != "" {
		pendingPtr = &pending
	}
	return QuotaSnapshot{StoredBytes: stored, DailyBytes: daily, MaxInputMedia: effective.MaxInputMediaBytes, MaxArchive: effective.MaxArchiveBytes, MaxOutputMedia: effective.MaxOutputMediaBytes, MaxFiles: effective.MaxFilesPerMessage, MaxUserStored: effective.MaxUserStoredBytes, MaxUserDaily: effective.MaxUserDailyBytes, MaxTotal: effective.MaxTotalBytes, MinFree: s.Limits.MinFreeBytes, Retention: effective.Retention, PendingDelete: pendingPtr}, nil
}

func (s Service) ScheduleDeleteAll(ctx context.Context, userID string, now time.Time) (model.AttachmentDeleteRequest, error) {
	return attachmentstore.CreateDeleteRequest(ctx, s.DB, userID, now, now.Add(deleteGracePeriod))
}

func (s Service) CancelDeleteAll(ctx context.Context, userID string) error {
	if err := attachmentstore.CancelDeleteRequest(ctx, s.DB, userID); errors.Is(err, attachmentstore.ErrDeleteRequestNotFound) {
		return ErrDeleteRequestNotFound
	} else {
		return err
	}
}

func (s Service) RunDueDeletes(ctx context.Context, now time.Time, batchSize int) error {
	requests, err := attachmentstore.ListDueDeleteRequests(ctx, s.DB, now, batchSize)
	if err != nil {
		return err
	}
	for _, request := range requests {
		if err := s.runDelete(ctx, request); err != nil {
			if s.Logger != nil {
				s.Logger.ErrorContext(ctx, "delete scheduled attachments", "request_id", request.ID, "error", err)
			}
			continue
		}
	}
	return nil
}

func (s Service) runDelete(ctx context.Context, request model.AttachmentDeleteRequest) error {
	paths, err := attachmentstore.ListUserAttachmentFiles(ctx, s.DB, request.UserID)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	deleted, err := attachmentstore.DeleteUserAttachmentMessages(ctx, s.DB, request.UserID)
	if err != nil {
		return err
	}
	if s.Publisher != nil {
		for _, message := range deleted {
			if err := s.Publisher.PublishMessageDeleted(ctx, message.GroupID, message.MessageID); err != nil {
				return err
			}
		}
	}
	return attachmentstore.DeleteDeleteRequest(ctx, s.DB, request.ID)
}
