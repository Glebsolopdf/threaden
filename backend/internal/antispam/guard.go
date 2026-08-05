package antispam

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"voice-rooms/internal/abuse"
	"voice-rooms/internal/model"
)

var (
	ErrMessageTooLong     = errors.New("message is too long")
	ErrMessageWarning     = errors.New("message warning")
	ErrMessageRateLimited = errors.New("message rate limited")
	linkPattern           = regexp.MustCompile(`(?i)\bhttps?://|\bwww\.`)
	mentionPattern        = regexp.MustCompile(`(^|\s)@[\pL\pN_]{2,}`)
)

type Store interface {
	ReserveIdempotencyKey(context.Context, string, string, string, string, time.Time, time.Duration) (string, bool, error)
	Messages(context.Context, string, time.Time, int, string) ([]model.GroupMessage, error)
	CreateGroupSpamWarning(context.Context, string, string, time.Time, int, int, time.Duration, time.Duration) (int, bool, error)
	DeleteRecentMessagesByAuthor(context.Context, string, string, time.Time, int) (int, error)
	DeleteRecentRepeatedMessages(context.Context, string, string, string, time.Time, int) (int, error)
}

type Limiter interface {
	Allow(context.Context, string, string, abuse.Limit) (abuse.Decision, error)
}

type Guard struct {
	limiter Limiter
	store   Store
	cfg     abuse.Config
	now     func() time.Time
	mu      sync.Mutex
	warned  map[string]time.Time
}

func NewGuard(limiter Limiter, store Store, cfg abuse.Config) *Guard {
	return &Guard{limiter: limiter, store: store, cfg: cfg, now: time.Now, warned: map[string]time.Time{}}
}

func (g *Guard) Check(ctx context.Context, groupID string, user model.User, body string) (Result, error) {
	if utf8.RuneCountInString(body) > g.cfg.MaxMessageRunes {
		return Result{}, ErrMessageTooLong
	}
	now := g.now().UTC()
	normalized := normalize(body)
	if normalized != "" {
		deleted, err := g.store.DeleteRecentRepeatedMessages(ctx, groupID, user.ID, normalized, now.Add(-g.cfg.MessageLimit.Refill), recentPurgeLimit)
		if err != nil {
			return Result{}, err
		}
		if deleted > 0 {
			result, err := g.recordGroupWarning(ctx, groupID, body, reasonRepeatedMessages, now)
			if err != nil {
				return Result{}, err
			}
			return result, g.warnOnce(user.ID, now)
		}
	}
	if recent, err := g.store.Messages(ctx, groupID, now.Add(-g.cfg.MessageLimit.Refill), recentScanLimit, ""); err != nil {
		return Result{}, err
	} else if similar, users := countSimilarMessages(body, recent); similar >= minSimilarMessages && users >= minSimilarUsers {
		return g.punish(ctx, groupID, user.ID, body, reasonNearDuplicateMessages, now)
	}
	if looksLikeContentSpam(body, g.cfg) {
		return g.punish(ctx, groupID, user.ID, body, reasonContentSpam, now)
	}
	limit := g.cfg.MessageLimit
	if now.Sub(user.CreatedAt) < g.cfg.NewAccountAge {
		limit.Capacity = min(limit.Capacity, g.cfg.NewAccountMessageCap)
	}
	if decision, err := g.limiter.Allow(ctx, "message:user", abuse.Subject("user", user.ID), limit); err != nil {
		return Result{}, err
	} else if !decision.Allowed {
		return g.punish(ctx, groupID, user.ID, body, reasonMessageRateLimit, now)
	}
	groupLimit := abuse.Limit{Capacity: limit.Capacity * 2, Refill: limit.Refill}
	if decision, err := g.limiter.Allow(ctx, "message:group", groupID, groupLimit); err != nil {
		return Result{}, err
	} else if !decision.Allowed {
		return g.punish(ctx, groupID, user.ID, body, reasonGroupRateLimit, now)
	}
	g.clearWarning(user.ID)
	return Result{}, nil
}

func (g *Guard) punish(ctx context.Context, groupID, userID, body, reason string, now time.Time) (Result, error) {
	if _, err := g.store.DeleteRecentMessagesByAuthor(ctx, groupID, userID, now.Add(-g.cfg.MessageLimit.Refill), recentPurgeLimit); err != nil {
		return Result{}, err
	}
	result, err := g.recordGroupWarning(ctx, groupID, body, reason, now)
	if err != nil {
		return Result{}, err
	}
	return result, g.warnOnce(userID, now)
}

func (g *Guard) Reserve(ctx context.Context, groupID, userID, key, responseID string) (string, bool, error) {
	if strings.TrimSpace(key) == "" {
		return responseID, true, nil
	}
	return g.store.ReserveIdempotencyKey(ctx, "message:"+groupID, userID, key, responseID, g.now().UTC(), g.cfg.IdempotencyTTL)
}

func looksLikeContentSpam(body string, cfg abuse.Config) bool {
	if countMatches(linkPattern, body) > cfg.MaxLinks || countMatches(mentionPattern, body) > cfg.MaxMentions {
		return true
	}
	return strings.Count(body, "\n") > 40 || hasRepeatedLine(body) || dominantRepeatedRune(body)
}

func countMatches(pattern *regexp.Regexp, body string) int {
	return len(pattern.FindAllStringIndex(body, -1))
}

func hasRepeatedLine(body string) bool {
	seen := map[string]int{}
	for _, line := range strings.Split(body, "\n") {
		line = normalize(line)
		if len(line) < 8 {
			continue
		}
		seen[line]++
		if seen[line] >= 4 {
			return true
		}
	}
	return false
}

func dominantRepeatedRune(body string) bool {
	var last rune
	run := 0
	for _, r := range body {
		if r == last {
			run++
			if run > 24 {
				return true
			}
			continue
		}
		last, run = r, 1
	}
	return false
}

func normalize(body string) string { return strings.ToLower(strings.TrimSpace(body)) }

func (g *Guard) warnOnce(userID string, now time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for id, until := range g.warned {
		if !until.After(now) {
			delete(g.warned, id)
		}
	}
	if until, ok := g.warned[userID]; ok && until.After(now) {
		return ErrMessageRateLimited
	}
	g.warned[userID] = now.Add(g.cfg.MessageLimit.Refill)
	return ErrMessageWarning
}

func (g *Guard) clearWarning(userID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.warned, userID)
}
