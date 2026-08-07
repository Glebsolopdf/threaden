package groups

import (
	"context"
	"log/slog"
	"time"
)

type CleanupConfig struct {
	InactiveAfter time.Duration
	DeleteGrace   time.Duration
	BatchSize     int
	DryRun        bool
	Logger        *slog.Logger
}

type EmergencyCleanupConfig struct {
	InactiveAfter time.Duration
	MessageMinAge time.Duration
	BatchSize     int
	Logger        *slog.Logger
}

type EmergencyCleanupStats struct {
	InactiveGroupsDeleted int
	OldMessagesDeleted    int
}

func DefaultCleanupConfig() CleanupConfig {
	return CleanupConfig{
		InactiveAfter: 7 * 24 * time.Hour,
		DeleteGrace:   24 * time.Hour,
		BatchSize:     20,
		DryRun:        true,
	}
}

func (s *Service) cleanupInactiveGroups(ctx context.Context) {
	cfg := s.cleanup
	if cfg.BatchSize < 1 {
		cfg.BatchSize = 20
	}
	if cfg.InactiveAfter <= 0 {
		cfg.InactiveAfter = 7 * 24 * time.Hour
	}
	if cfg.DeleteGrace <= 0 {
		cfg.DeleteGrace = 24 * time.Hour
	}
	now := s.now().UTC()
	candidates, err := s.store.InactiveGroupCandidates(ctx, now.Add(-cfg.InactiveAfter), cfg.BatchSize)
	if err != nil {
		logGroupCleanup(cfg, ctx, slog.LevelError, "list inactive groups", "error", err)
		return
	}
	if len(candidates) == 0 {
		return
	}
	for _, item := range candidates {
		logGroupCleanup(cfg, ctx, slog.LevelInfo, "inactive group candidate",
			"group_id", item.ID,
			"reason", item.Reason,
			"messages", item.MessageCount,
			"estimated_bytes", item.EstimatedStoredBytes,
			"scheduled_for_deletion_at", item.ScheduledForDeletion)
	}
	if cfg.DryRun {
		logGroupCleanup(cfg, ctx, slog.LevelInfo, "inactive group cleanup dry run", "count", len(candidates))
		return
	}
	count, err := s.store.ScheduleInactiveGroups(ctx, now.Add(-cfg.InactiveAfter), now.Add(cfg.DeleteGrace), cfg.BatchSize)
	if err != nil {
		logGroupCleanup(cfg, ctx, slog.LevelError, "schedule inactive groups", "error", err)
		return
	}
	deleted, err := s.store.DeleteScheduledGroups(ctx, now, now.Add(-cfg.InactiveAfter), cfg.BatchSize)
	if err != nil {
		logGroupCleanup(cfg, ctx, slog.LevelError, "delete scheduled groups", "error", err)
		return
	}
	logGroupCleanup(cfg, ctx, slog.LevelInfo, "inactive group cleanup completed",
		"scheduled", count, "deleted", len(deleted))
}

func (s *Service) EmergencyCleanup(ctx context.Context, cfg EmergencyCleanupConfig) EmergencyCleanupStats {
	if cfg.BatchSize < 1 {
		cfg.BatchSize = 500
	}
	if cfg.InactiveAfter <= 0 {
		cfg.InactiveAfter = 7 * 24 * time.Hour
	}
	if cfg.MessageMinAge <= 0 {
		cfg.MessageMinAge = 24 * time.Hour
	}
	if cfg.Logger == nil {
		cfg.Logger = s.cleanup.Logger
	}
	now := s.now().UTC()
	_ = s.store.DeleteExpiredMessages(ctx, now.Add(-7*24*time.Hour))
	groups, err := s.store.DeleteInactiveGroupsNow(ctx, now.Add(-cfg.InactiveAfter), cfg.BatchSize)
	if err != nil {
		logEmergencyCleanup(cfg, ctx, slog.LevelError, "emergency delete inactive groups", "error", err)
	}
	messages, err := s.store.DeleteOldestMessages(ctx, now.Add(-cfg.MessageMinAge), cfg.BatchSize)
	if err != nil {
		logEmergencyCleanup(cfg, ctx, slog.LevelError, "emergency delete old messages", "error", err)
	}
	stats := EmergencyCleanupStats{InactiveGroupsDeleted: len(groups), OldMessagesDeleted: messages}
	logEmergencyCleanup(cfg, ctx, slog.LevelWarn, "low disk emergency cleanup completed",
		"inactive_groups_deleted", stats.InactiveGroupsDeleted,
		"old_messages_deleted", stats.OldMessagesDeleted)
	return stats
}

func logGroupCleanup(cfg CleanupConfig, ctx context.Context, level slog.Level, message string, args ...any) {
	if cfg.Logger == nil {
		return
	}
	cfg.Logger.Log(ctx, level, message, args...)
}

func logEmergencyCleanup(cfg EmergencyCleanupConfig, ctx context.Context, level slog.Level, message string, args ...any) {
	if cfg.Logger == nil {
		return
	}
	cfg.Logger.Log(ctx, level, message, args...)
}
