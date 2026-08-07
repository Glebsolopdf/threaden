package abuse

import (
	"context"
	"fmt"
	"time"
)

type LimiterStore interface {
	TakeRateLimit(context.Context, string, time.Time, int, float64, time.Duration) (bool, time.Duration, error)
	NoteViolation(context.Context, string, time.Time, time.Duration, int, []time.Duration, time.Duration) (int, time.Time, error)
	IPBanActive(context.Context, string, time.Time) (bool, time.Time, error)
}

type Limiter struct {
	store LimiterStore
	cfg   Config
	now   func() time.Time
}

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
	Key        string
}

// BanResult describes a newly created ban: its escalation level (1-based) and
// when it expires. A zero Level means no ban was created.
type BanResult struct {
	Level int
	Until time.Time
}

func NewLimiter(store LimiterStore, cfg Config) *Limiter {
	return &Limiter{store: store, cfg: cfg, now: time.Now}
}

func (l *Limiter) Allow(ctx context.Context, scope, subject string, limit Limit) (Decision, error) {
	key := scope + ":" + subject
	ok, retry, err := l.store.TakeRateLimit(
		ctx, key, l.now().UTC(), limit.Capacity, RefillPerSecond(limit), l.cfg.BucketTTL,
	)
	if err != nil {
		return Decision{}, err
	}
	return Decision{Allowed: ok, RetryAfter: retry, Key: key}, nil
}

func (l *Limiter) NoteViolation(ctx context.Context, ip string) (BanResult, error) {
	level, until, err := l.store.NoteViolation(
		ctx, banKey(ip), l.now().UTC(), l.cfg.IPBanWindow, l.cfg.IPBanThreshold,
		l.cfg.IPBanSteps, l.cfg.IPBanEscalationForget,
	)
	if err != nil {
		return BanResult{}, err
	}
	return BanResult{Level: level, Until: until}, nil
}

func (l *Limiter) Banned(ctx context.Context, ip string) (bool, time.Time, error) {
	return l.store.IPBanActive(ctx, banKey(ip), l.now().UTC())
}

func banKey(ip string) string { return "ip:" + ip }

func EndpointLimit(cfg Config, route string) Limit {
	switch route {
	case "POST /v1/auth/register":
		return cfg.RegisterLimit
	case "POST /v1/auth/login":
		return cfg.LoginLimit
	case "POST /v1/groups":
		return cfg.GroupCreateLimit
	case "POST /v1/groups/{id}/members", "POST /v1/invites/{token}/join",
		"POST /v1/rooms/{code}/join", "POST /v1/group-voice-rooms/{id}/join":
		return cfg.GroupJoinLimit
	case "POST /v1/groups/{id}/messages":
		return cfg.MessageLimit
	case "PATCH /v1/me":
		return cfg.ProfileUpdateLimit
	case "DELETE /v1/me/avatar":
		return cfg.ProfileUpdateLimit
	case "GET /v1/discover/groups":
		return cfg.SearchLimit
	case "GET /v1/welcome":
		return cfg.HeavyLimit
	case "GET /readyz", "GET /v1/events":
		return cfg.HeavyLimit
	default:
		return cfg.GlobalLimit
	}
}

func Subject(kind, value string) string {
	if value == "" {
		value = "anonymous"
	}
	return fmt.Sprintf("%s:%s", kind, value)
}
