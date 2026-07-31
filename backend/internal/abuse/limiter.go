package abuse

import (
	"context"
	"fmt"
	"time"
)

type BucketStore interface {
	TakeRateLimit(context.Context, string, time.Time, int, float64, time.Duration) (bool, time.Duration, error)
}

type Limiter struct {
	store BucketStore
	cfg   Config
	now   func() time.Time
}

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
	Key        string
}

func NewLimiter(store BucketStore, cfg Config) *Limiter {
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
