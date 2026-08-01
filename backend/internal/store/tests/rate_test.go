package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	store "voice-rooms/internal/store"
)

func TestTakeRateLimitRefillsAndDenies(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "rate.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	for i := 0; i < 2; i++ {
		ok, _, err := st.TakeRateLimit(ctx, "test:1", now, 2, 0.1, time.Hour)
		if err != nil || !ok {
			t.Fatalf("token %d: ok=%v err=%v", i, ok, err)
		}
	}
	ok, retry, err := st.TakeRateLimit(ctx, "test:1", now, 2, 0.1, time.Hour)
	if err != nil || ok || retry <= 0 {
		t.Fatalf("third token must be denied: ok=%v retry=%v err=%v", ok, retry, err)
	}
	ok, _, err = st.TakeRateLimit(ctx, "test:1", now.Add(10*time.Second), 2, 0.1, time.Hour)
	if err != nil || !ok {
		t.Fatalf("refilled token must be allowed: ok=%v err=%v", ok, err)
	}
}

func TestNoteViolationBansAndExpires(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "bans.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	key := "ip:198.51.100.7"
	steps := []time.Duration{24 * time.Hour}
	level, _, err := st.NoteViolation(ctx, key, now, 15*time.Minute, 3, steps, 24*time.Hour)
	if err != nil || level != 0 {
		t.Fatalf("first violation: level=%d err=%v", level, err)
	}
	level, _, _ = st.NoteViolation(ctx, key, now, 15*time.Minute, 3, steps, 24*time.Hour)
	if level != 0 {
		t.Fatal("second violation must not ban yet")
	}
	level, _, err = st.NoteViolation(ctx, key, now, 15*time.Minute, 3, steps, 24*time.Hour)
	if err != nil || level != 1 {
		t.Fatalf("third violation must ban at level 1: level=%d err=%v", level, err)
	}
	active, until, err := st.IPBanActive(ctx, key, now)
	if err != nil || !active || until.Before(now.Add(23*time.Hour)) {
		t.Fatalf("ban not active: active=%v until=%v err=%v", active, until, err)
	}
	if level, _, _ = st.NoteViolation(ctx, key, now.Add(time.Minute), 15*time.Minute, 3, steps, 24*time.Hour); level != 0 {
		t.Fatal("violation during active ban must be ignored")
	}
	after := now.Add(24*time.Hour + time.Minute)
	if active, _, err = st.IPBanActive(ctx, key, after); err != nil || active {
		t.Fatalf("ban must expire after duration: active=%v err=%v", active, err)
	}
	if level, _, _ = st.NoteViolation(ctx, key, after, 15*time.Minute, 3, steps, 24*time.Hour); level != 0 {
		t.Fatal("violation after expiry must start a fresh window")
	}
}
