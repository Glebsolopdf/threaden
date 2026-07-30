package config

import (
	"strings"
	"testing"
)

func TestLoadValidation(t *testing.T) {
	t.Setenv("ROOM_TTL", "nope")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "ROOM_TTL") {
		t.Fatalf("expected ROOM_TTL error, got %v", err)
	}

	t.Setenv("ROOM_TTL", "1h")
	t.Setenv("SESSION_TTL", "1h")
	t.Setenv("SESSION_IDLE_TTL", "2h")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SESSION_IDLE_TTL") {
		t.Fatalf("expected session policy error, got %v", err)
	}
	t.Setenv("SESSION_IDLE_TTL", "30m")
	t.Setenv("MAX_ROOM_PARTICIPANTS", "0")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "MAX_ROOM_PARTICIPANTS") {
		t.Fatalf("expected participant limit error, got %v", err)
	}

	t.Setenv("MAX_ROOM_PARTICIPANTS", "16")
	t.Setenv("LOW_DISK_MIN_FREE_BYTES", "0")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "LOW_DISK_MIN_FREE_BYTES") {
		t.Fatalf("expected low disk threshold error, got %v", err)
	}
}
