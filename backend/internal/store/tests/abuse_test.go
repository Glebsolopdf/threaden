package store_test

import (
	"context"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	"voice-rooms/internal/model"
	store "voice-rooms/internal/store"
)

func TestBanEscalationLadder(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "ladder.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	key := "ip:203.0.113.9"
	steps := []time.Duration{10 * time.Second, time.Minute, 5 * time.Minute, 24 * time.Hour}

	cases := []struct {
		at    time.Time
		level int
		until time.Time
	}{
		{now, 1, now.Add(10 * time.Second)},
		{now.Add(time.Second), 0, time.Time{}},
		{now.Add(11 * time.Second), 2, now.Add(11*time.Second + time.Minute)},
		{now.Add(72 * time.Second), 3, now.Add(72*time.Second + 5*time.Minute)},
		{now.Add(373 * time.Second), 4, now.Add(373*time.Second + 24*time.Hour)},
		{now.Add(374 * time.Second), 0, time.Time{}},
	}
	for _, tc := range cases {
		level, until, err := st.NoteViolation(ctx, key, tc.at, 15*time.Minute, 1, steps, 24*time.Hour)
		if err != nil {
			t.Fatalf("violation at %v: %v", tc.at, err)
		}
		if level != tc.level || !until.Equal(tc.until) {
			t.Fatalf("violation at %v: level=%d until=%v, want level=%d until=%v", tc.at, level, until, tc.level, tc.until)
		}
	}

	after := now.Add(373*time.Second + 24*time.Hour + time.Second)
	level, _, err := st.NoteViolation(ctx, key, after, 15*time.Minute, 1, steps, 24*time.Hour)
	if err != nil || level != 1 {
		t.Fatalf("escalation must be forgotten after 24h without violations: level=%d err=%v", level, err)
	}
}

func TestBanEscalationForgetsAfterQuietWindow(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "forget.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	key := "ip:203.0.113.10"
	steps := []time.Duration{10 * time.Second, time.Minute, 5 * time.Minute, 24 * time.Hour}

	level, _, err := st.NoteViolation(ctx, key, now, 15*time.Minute, 1, steps, 24*time.Hour)
	if err != nil || level != 1 {
		t.Fatalf("first ban: level=%d err=%v", level, err)
	}
	level, _, err = st.NoteViolation(ctx, key, now.Add(24*time.Hour+time.Minute), 15*time.Minute, 1, steps, 24*time.Hour)
	if err != nil || level != 1 {
		t.Fatalf("next ban after 24h of quiet must start over at level 1: level=%d err=%v", level, err)
	}
}

func TestRecordAccountBanWindowedCount(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "account.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	user := model.User{ID: "usr_account", Email: "ban@example.com", DisplayName: "Ban", CreatedAt: now}
	if err := st.CreateUser(ctx, user, []byte("hashed-password"), sha256.Sum256([]byte("account-token"))); err != nil {
		t.Fatal(err)
	}
	window := 30 * 24 * time.Hour
	checks := []struct {
		at    time.Time
		count int
	}{
		{now, 1},
		{now.Add(24 * time.Hour), 2},
		{now.Add(29 * 24 * time.Hour), 3},
		{now.Add(31 * 24 * time.Hour), 3},
		{now.Add(61 * 24 * time.Hour), 2},
		{now.Add(61*24*time.Hour + time.Hour), 2},
	}
	for _, tc := range checks {
		count, err := st.RecordAccountBan(ctx, user.ID, tc.at, window)
		if err != nil {
			t.Fatalf("record at %v: %v", tc.at, err)
		}
		if count != tc.count {
			t.Fatalf("record at %v: count=%d, want %d", tc.at, count, tc.count)
		}
	}
}

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
