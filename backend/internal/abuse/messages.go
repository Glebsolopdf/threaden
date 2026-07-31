package abuse

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"voice-rooms/internal/model"
)

var (
	ErrMessageTooLong     = errors.New("message is too long")
	ErrMessageSpam        = errors.New("message looks like spam")
	ErrMessageRateLimited = errors.New("message rate limited")
	linkPattern           = regexp.MustCompile(`(?i)\bhttps?://|\bwww\.`)
	mentionPattern        = regexp.MustCompile(`(^|\s)@[\pL\pN_]{2,}`)
)

type IdempotencyStore interface {
	ReserveIdempotencyKey(context.Context, string, string, string, string, time.Time, time.Duration) (string, bool, error)
}

type MessageGuard struct {
	limiter *Limiter
	store   IdempotencyStore
	cfg     Config
	now     func() time.Time
}

func NewMessageGuard(limiter *Limiter, store IdempotencyStore, cfg Config) *MessageGuard {
	return &MessageGuard{limiter: limiter, store: store, cfg: cfg, now: time.Now}
}

func (g *MessageGuard) Check(ctx context.Context, groupID string, user model.User, body string) error {
	if utf8.RuneCountInString(body) > g.cfg.MaxMessageRunes {
		return ErrMessageTooLong
	}
	if linkPattern.FindAllStringIndex(body, -1) != nil && countMatches(linkPattern, body) > g.cfg.MaxLinks {
		return ErrMessageSpam
	}
	if countMatches(mentionPattern, body) > g.cfg.MaxMentions || hasRepeatedLine(body) {
		return ErrMessageSpam
	}
	if strings.Count(body, "\n") > 40 || dominantRepeatedRune(body) {
		return ErrMessageSpam
	}
	now := g.now().UTC()
	limit := g.cfg.MessageLimit
	if now.Sub(user.CreatedAt) < g.cfg.NewAccountAge {
		limit.Capacity = min(limit.Capacity, g.cfg.NewAccountMessageCap)
	}
	if decision, err := g.limiter.Allow(ctx, "message:user", Subject("user", user.ID), limit); err != nil {
		return err
	} else if !decision.Allowed {
		return ErrMessageRateLimited
	}
	groupLimit := Limit{Capacity: limit.Capacity * 2, Refill: limit.Refill}
	if decision, err := g.limiter.Allow(ctx, "message:group", groupID, groupLimit); err != nil {
		return err
	} else if !decision.Allowed {
		return ErrMessageRateLimited
	}
	return nil
}

func (g *MessageGuard) Reserve(ctx context.Context, groupID, userID, key, responseID string) (string, bool, error) {
	if strings.TrimSpace(key) == "" {
		return responseID, true, nil
	}
	scope := "message:" + groupID
	return g.store.ReserveIdempotencyKey(ctx, scope, userID, key, responseID, g.now().UTC(), g.cfg.IdempotencyTTL)
}

func countMatches(pattern *regexp.Regexp, body string) int {
	return len(pattern.FindAllStringIndex(body, -1))
}

func hasRepeatedLine(body string) bool {
	seen := map[string]int{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
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
