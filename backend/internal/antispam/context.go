package antispam

import (
	"context"
	"time"
)

const (
	groupSpamWindow       = 30 * 24 * time.Hour
	groupSpamWarningLimit = 3
)

type Result struct {
	DeleteGroup bool
}

func (g *Guard) evaluateGroup(ctx context.Context, groupID, body, reason string, now time.Time) (Result, error) {
	window := g.cfg.MessageLimit.Refill
	if window <= 0 {
		window = time.Minute
	}
	messages, err := g.store.Messages(ctx, groupID, now.Add(-window), 40, "")
	if err != nil {
		return Result{}, err
	}
	similar, users := countSimilarMessages(body, messages)
	if similar < 3 || users < 2 {
		return Result{}, nil
	}
	warnings, created, err := g.store.CreateGroupSpamWarning(ctx, groupID, reason, now, similar, users, window, groupSpamWindow)
	if err != nil || !created {
		return Result{}, err
	}
	return Result{DeleteGroup: warnings >= groupSpamWarningLimit}, nil
}
