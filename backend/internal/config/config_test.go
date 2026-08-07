package config

import (
	"os"
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

func TestLoadUsesIPv4LoopbackForPublicLiveKit(t *testing.T) {
	previous, existed := os.LookupEnv("LIVEKIT_PUBLIC_URL")
	if err := os.Unsetenv("LIVEKIT_PUBLIC_URL"); err != nil {
		t.Fatalf("unset LIVEKIT_PUBLIC_URL: %v", err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv("LIVEKIT_PUBLIC_URL", previous)
		} else {
			_ = os.Unsetenv("LIVEKIT_PUBLIC_URL")
		}
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	if cfg.LiveKitPublicURL != "ws://127.0.0.1:7880" {
		t.Fatalf("unexpected public LiveKit URL: %q", cfg.LiveKitPublicURL)
	}
}

func TestLoadEnablesInactiveGroupDeletionByDefault(t *testing.T) {
	previous, existed := os.LookupEnv("GROUP_CLEANUP_DRY_RUN")
	if err := os.Unsetenv("GROUP_CLEANUP_DRY_RUN"); err != nil {
		t.Fatalf("unset GROUP_CLEANUP_DRY_RUN: %v", err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv("GROUP_CLEANUP_DRY_RUN", previous)
		} else {
			_ = os.Unsetenv("GROUP_CLEANUP_DRY_RUN")
		}
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	if cfg.GroupCleanupDryRun {
		t.Fatal("inactive group cleanup must not default to dry-run")
	}
}
