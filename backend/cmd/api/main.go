package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voice-rooms/internal/abuse"
	"voice-rooms/internal/antispam"
	"voice-rooms/internal/app"
	"voice-rooms/internal/config"
	"voice-rooms/internal/disk"
	appgroups "voice-rooms/internal/groups"
	"voice-rooms/internal/groups/hub"
	"voice-rooms/internal/httpapi"
	livekitgateway "voice-rooms/internal/livekit"
	"voice-rooms/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	st, err := store.Open(cfg.DatabasePath)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	voice := livekitgateway.New(
		cfg.LiveKitURL, cfg.LiveKitPublicURL, cfg.LiveKitAPIKey, cfg.LiveKitAPISecret,
	)
	service := app.New(
		st, voice, cfg.RoomTTL, cfg.LiveKitTokenTTL,
		cfg.MaxRoomParticipants, logger,
	).WithSessionPolicy(cfg.SessionTTL, cfg.SessionIdleTTL)
	security := abuse.DefaultConfig()
	security.TrustedProxies = cfg.TrustedProxies
	security.BucketTTL = cfg.RateLimitBucketTTL
	security.MaxMessageRunes = cfg.MaxMessageRunes
	security.MaxLinks = cfg.MaxMessageLinks
	security.MaxMentions = cfg.MaxMessageMentions
	security.IPBanThreshold = cfg.IPBanThreshold
	security.IPBanWindow = cfg.IPBanWindow
	security.IPBanSteps = cfg.IPBanSteps
	security.IPBanEscalationForget = cfg.IPBanEscalationForget
	security.AccountBanWindow = cfg.AccountBanWindow
	security.AccountBanDeletionCount = cfg.AccountBanDeletionCount
	limiter := abuse.NewLimiter(st, security)
	groupService := appgroups.New(st, voice, hub.New()).
		WithLimits(appgroups.Limits{MaxUserGroups: cfg.MaxUserGroups, DiscoverMinMembers: cfg.DiscoverMinMembers}).
		WithMessageGuard(antispam.NewGuard(limiter, st, security)).
		WithCleanup(appgroups.CleanupConfig{
			InactiveAfter: cfg.InactiveGroupTTL,
			DeleteGrace:   cfg.GroupDeleteGrace,
			BatchSize:     cfg.GroupCleanupBatch,
			DryRun:        cfg.GroupCleanupDryRun,
			Logger:        logger,
		})
	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewWithOptions(service, groupService, st, logger, httpapi.Options{
			Origins: cfg.CORSAllowedOrigins, Security: security, SessionCookieSecure: cfg.SessionCookieSecure,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cleanupDone := make(chan struct{})
	go func() {
		defer close(cleanupDone)
		service.RunCleanup(runCtx, time.Minute)
	}()
	go func() {
		groupService.Cleanup(runCtx)
		ticker := time.NewTicker(cfg.GroupCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				groupService.Cleanup(runCtx)
			}
		}
	}()
	go runLowDiskMonitor(runCtx, logger, disk.NewChecker(cfg.DatabasePath), service, groupService, cfg)
	go func() {
		<-runCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown", "error", err)
		}
	}()

	logger.Info("server starting", "address", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server stopped", "error", err)
	}
	stop()
	select {
	case <-cleanupDone:
	case <-time.After(10 * time.Second):
		logger.Warn("cleanup did not stop before timeout")
	}
	logger.Info("server stopped")
}

func runLowDiskMonitor(
	ctx context.Context,
	logger *slog.Logger,
	checker disk.Checker,
	service *app.Service,
	groupService *appgroups.Service,
	cfg config.Config,
) {
	check := func() {
		free, err := checker.FreeBytes()
		if err != nil {
			logger.ErrorContext(ctx, "check free disk space", "error", err)
			return
		}
		if free >= cfg.LowDiskMinFreeBytes {
			return
		}
		logger.WarnContext(ctx, "low disk space emergency cleanup started",
			"free_bytes", free, "min_free_bytes", cfg.LowDiskMinFreeBytes)
		totalUsers, totalGroups, totalMessages := 0, 0, 0
		after := free
		for pass := 1; pass <= 10 && after < cfg.LowDiskMinFreeBytes; pass++ {
			appStats := service.EmergencyCleanup(ctx, cfg.LowDiskCleanupBatch)
			groupStats := groupService.EmergencyCleanup(ctx, appgroups.EmergencyCleanupConfig{
				InactiveAfter: cfg.InactiveGroupTTL,
				MessageMinAge: cfg.LowDiskMessageMinAge,
				BatchSize:     cfg.LowDiskCleanupBatch,
				Logger:        logger,
			})
			totalUsers += appStats.InactiveUsersDeleted
			totalGroups += groupStats.InactiveGroupsDeleted
			totalMessages += groupStats.OldMessagesDeleted
			after, err = checker.FreeBytes()
			if err != nil {
				logger.ErrorContext(ctx, "check free disk space after cleanup", "error", err)
				return
			}
			if appStats.InactiveUsersDeleted+groupStats.InactiveGroupsDeleted+groupStats.OldMessagesDeleted == 0 {
				break
			}
		}
		logger.WarnContext(ctx, "low disk space emergency cleanup finished",
			"free_bytes_before", free,
			"free_bytes_after", after,
			"inactive_users_deleted", totalUsers,
			"inactive_groups_deleted", totalGroups,
			"old_messages_deleted", totalMessages)
	}
	check()
	ticker := time.NewTicker(cfg.LowDiskCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}
