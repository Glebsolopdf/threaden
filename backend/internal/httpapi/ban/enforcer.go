// Package ban enforces the account-level consequences of IP abuse bans at the
// HTTP boundary: a ban at the highest escalation level is attributed to the
// authenticated account that triggered it, and accounts that accumulate
// AccountBanDeletionCount such bans within AccountBanWindow are deleted.
package ban

import (
	"context"
	"log/slog"
	"time"

	"voice-rooms/internal/abuse"
	"voice-rooms/internal/app"
	"voice-rooms/internal/store"
)

type Enforcer struct {
	limiter *abuse.Limiter
	service *app.Service
	st      *store.Store
	cfg     abuse.Config
	logger  *slog.Logger
	now     func() time.Time
}

func NewEnforcer(
	limiter *abuse.Limiter,
	service *app.Service,
	st *store.Store,
	cfg abuse.Config,
	logger *slog.Logger,
) *Enforcer {
	return &Enforcer{limiter: limiter, service: service, st: st, cfg: cfg, logger: logger, now: time.Now}
}

func (e *Enforcer) Banned(ctx context.Context, ip string) (bool, time.Time, error) {
	return e.limiter.Banned(ctx, ip)
}

// NoteViolation records a rate-limit violation for the IP and, when a ban at
// the highest escalation level was created, attributes it to the account that
// caused it. Accounts that reach AccountBanDeletionCount such bans within
// AccountBanWindow are automatically deleted.
func (e *Enforcer) NoteViolation(ctx context.Context, token, ip string) error {
	result, err := e.limiter.NoteViolation(ctx, ip)
	if err != nil {
		return err
	}
	return e.record(ctx, token, result)
}

func (e *Enforcer) record(ctx context.Context, token string, result abuse.BanResult) error {
	if result.Level == 0 || token == "" {
		return nil
	}
	user, err := e.service.Authenticate(ctx, token)
	if err != nil {
		return nil
	}
	if err := e.service.CleanupBannedUser(ctx, user); err != nil {
		return err
	}
	if result.Level < len(e.cfg.IPBanSteps) {
		return nil
	}
	count, err := e.st.RecordAccountBan(ctx, user.ID, e.now().UTC(), e.cfg.AccountBanWindow)
	if err != nil {
		return err
	}
	if count < e.cfg.AccountBanDeletionCount {
		return nil
	}
	e.logger.WarnContext(ctx, "auto-deleted account after repeated maximum-level bans",
		"user_id", user.ID, "bans", count, "threshold", e.cfg.AccountBanDeletionCount)
	return e.service.DeleteUser(ctx, user)
}
