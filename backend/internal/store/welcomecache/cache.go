package welcomecache

import (
	"sync"
	"time"

	"voice-rooms/internal/model"
)

const TTL = time.Hour

type Cache struct {
	mu        sync.Mutex
	stats     model.WelcomeStats
	updatedAt time.Time
}

func (c *Cache) Get(now time.Time, refresh func() (model.WelcomeStats, error)) (model.WelcomeStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.updatedAt.IsZero() && now.Sub(c.updatedAt) < TTL {
		return c.stats, nil
	}
	stats, err := refresh()
	if err != nil {
		return model.WelcomeStats{}, err
	}
	c.stats = stats
	c.updatedAt = now
	return stats, nil
}
