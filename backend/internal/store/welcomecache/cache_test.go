package welcomecache

import (
	"testing"
	"time"

	"voice-rooms/internal/model"
)

func TestGetRefreshesAtMostOncePerHour(t *testing.T) {
	var cache Cache
	now := time.Unix(1_700_000_000, 0).UTC()
	calls := 0
	refresh := func() (model.WelcomeStats, error) {
		calls++
		return model.WelcomeStats{Messages: calls}, nil
	}

	first, err := cache.Get(now, refresh)
	if err != nil || first.Messages != 1 {
		t.Fatalf("first refresh: %+v %v", first, err)
	}
	second, err := cache.Get(now.Add(59*time.Minute), refresh)
	if err != nil || second.Messages != 1 {
		t.Fatalf("cached result: %+v %v", second, err)
	}
	third, err := cache.Get(now.Add(time.Hour), refresh)
	if err != nil || third.Messages != 2 {
		t.Fatalf("hourly refresh: %+v %v", third, err)
	}
}
