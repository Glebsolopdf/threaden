package antispam

import (
	"context"
	"time"
)

const (
	groupSpamWindow       = 30 * 24 * time.Hour
	groupSpamWarningLimit = 3
	recentScanLimit       = 20
	recentPurgeLimit      = 20
	similarScanLimit      = 40
	minSimilarMessages    = 3
	minSimilarUsers       = 2
)

const (
	reasonRepeatedMessages      = "repeated_messages"
	reasonNearDuplicateMessages = "near_duplicate_messages"
	reasonContentSpam           = "content_spam"
	reasonMessageRateLimit      = "message_rate_limit"
	reasonGroupRateLimit        = "group_rate_limit"
)

type Result struct {
	DeleteGroup bool
}

func (g *Guard) recordGroupWarning(ctx context.Context, groupID, body, reason string, now time.Time) (Result, error) {
	window := g.cfg.MessageLimit.Refill
	if window <= 0 {
		window = time.Minute
	}
	messages, err := g.store.Messages(ctx, groupID, now.Add(-window), similarScanLimit, "")
	if err != nil {
		return Result{}, err
	}
	similar, users := countSimilarMessages(body, messages)
	if similar < minSimilarMessages || users < minSimilarUsers {
		return Result{}, nil
	}
	warnings, created, err := g.store.CreateGroupSpamWarning(ctx, groupID, reason, now, similar, users, window, groupSpamWindow)
	if err != nil || !created {
		return Result{}, err
	}
	return Result{DeleteGroup: warnings >= groupSpamWarningLimit}, nil
}
