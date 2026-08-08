package cleanup

import (
	"context"
	"log/slog"
	"time"

	"voice-rooms/internal/store"
)

type Config struct {
	InactiveAfter time.Duration
	DeleteGrace   time.Duration
	BatchSize     int
	DryRun        bool
	Logger        *slog.Logger
}

type EmergencyConfig struct {
	InactiveAfter time.Duration
	MessageMinAge time.Duration
	BatchSize     int
	Logger        *slog.Logger
}

type Stats struct {
	InactiveGroupsDeleted int
	OldMessagesDeleted    int
}

func DefaultConfig() Config {
	return Config{
		InactiveAfter: 7 * 24 * time.Hour,
		DeleteGrace:   24 * time.Hour,
		BatchSize:     20,
		DryRun:        true,
	}
}

func Run(ctx context.Context, st *store.Store, now time.Time, cfg Config) {
	if cfg.BatchSize < 1 {
		cfg.BatchSize = 20
	}
	if cfg.InactiveAfter <= 0 {
		cfg.InactiveAfter = 7 * 24 * time.Hour
	}
	if cfg.DeleteGrace <= 0 {
		cfg.DeleteGrace = 24 * time.Hour
	}
	now = now.UTC()
	candidates, err := st.InactiveGroupCandidates(ctx, now.Add(-cfg.InactiveAfter), cfg.BatchSize)
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
	count, err := st.ScheduleInactiveGroups(ctx, now.Add(-cfg.InactiveAfter), now.Add(cfg.DeleteGrace), cfg.BatchSize)
	if err != nil {
		logGroupCleanup(cfg, ctx, slog.LevelError, "schedule inactive groups", "error", err)
		return
	}
	deleted, err := st.DeleteScheduledGroups(ctx, now, now.Add(-cfg.InactiveAfter), cfg.BatchSize)
	if err != nil {
		logGroupCleanup(cfg, ctx, slog.LevelError, "delete scheduled groups", "error", err)
		return
	}
	logGroupCleanup(cfg, ctx, slog.LevelInfo, "inactive group cleanup completed",
		"scheduled", count, "deleted", len(deleted))
}

func Emergency(ctx context.Context, st *store.Store, now time.Time, cfg EmergencyConfig) Stats {
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
		cfg.Logger = nil
	}
	now = now.UTC()
	_ = st.DeleteExpiredMessages(ctx, now.Add(-7*24*time.Hour))
	groups, err := st.DeleteInactiveGroupsNow(ctx, now.Add(-cfg.InactiveAfter), cfg.BatchSize)
	if err != nil {
		logEmergencyCleanup(cfg, ctx, slog.LevelError, "emergency delete inactive groups", "error", err)
	}
	messages, err := st.DeleteOldestMessages(ctx, now.Add(-cfg.MessageMinAge), cfg.BatchSize)
	if err != nil {
		logEmergencyCleanup(cfg, ctx, slog.LevelError, "emergency delete old messages", "error", err)
	}
	stats := Stats{InactiveGroupsDeleted: len(groups), OldMessagesDeleted: messages}
	logEmergencyCleanup(cfg, ctx, slog.LevelWarn, "low disk emergency cleanup completed",
		"inactive_groups_deleted", stats.InactiveGroupsDeleted,
		"old_messages_deleted", stats.OldMessagesDeleted)
	return stats
}

func logGroupCleanup(cfg Config, ctx context.Context, level slog.Level, message string, args ...any) {
	if cfg.Logger == nil {
		return
	}
	cfg.Logger.Log(ctx, level, message, args...)
}

func logEmergencyCleanup(cfg EmergencyConfig, ctx context.Context, level slog.Level, message string, args ...any) {
	if cfg.Logger == nil {
		return
	}
	cfg.Logger.Log(ctx, level, message, args...)
}
