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
