package config

import (
	"os"
	"strings"
	"testing"
	"time"
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

func TestLoadRejectsDefaultLiveKitCredentialsForPublicURL(t *testing.T) {
	t.Setenv("LIVEKIT_PUBLIC_URL", "wss://livekit.example.com")
	t.Setenv("LIVEKIT_API_KEY", "devkey")
	t.Setenv("LIVEKIT_API_SECRET", "secret")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "LIVEKIT_API_KEY") {
		t.Fatalf("expected default LiveKit credential error, got %v", err)
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

func TestLoadAttachmentDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	if cfg.AttachmentMaxInputMediaBytes != 10*1024*1024 {
		t.Fatalf("unexpected media input limit: %d", cfg.AttachmentMaxInputMediaBytes)
	}
	if cfg.AttachmentMaxArchiveBytes != 5*1024*1024 {
		t.Fatalf("unexpected archive limit: %d", cfg.AttachmentMaxArchiveBytes)
	}
	if cfg.AttachmentMaxOutputMediaBytes != 1*1024*1024 {
		t.Fatalf("unexpected media output limit: %d", cfg.AttachmentMaxOutputMediaBytes)
	}
	if cfg.AttachmentMaxFilesPerMessage != 3 || cfg.AttachmentMaxUserStoredBytes != 50*1024*1024 {
		t.Fatalf("unexpected attachment count or user quota: %d, %d", cfg.AttachmentMaxFilesPerMessage, cfg.AttachmentMaxUserStoredBytes)
	}
	if cfg.AttachmentMaxUserDailyBytes != 20*1024*1024 || cfg.AttachmentMaxTotalBytes != 5*1024*1024*1024 {
		t.Fatalf("unexpected daily or total quota: %d, %d", cfg.AttachmentMaxUserDailyBytes, cfg.AttachmentMaxTotalBytes)
	}
	if cfg.AttachmentRetention != 72*time.Hour || cfg.AttachmentStorageDir != "./data/attachments" {
		t.Fatalf("unexpected retention or storage directory: %s, %q", cfg.AttachmentRetention, cfg.AttachmentStorageDir)
	}
}

func TestLoadAttachmentOverrides(t *testing.T) {
	t.Setenv("ATTACHMENT_MAX_INPUT_MEDIA_BYTES", "123")
	t.Setenv("ATTACHMENT_MAX_ARCHIVE_BYTES", "456")
	t.Setenv("ATTACHMENT_MAX_OUTPUT_MEDIA_BYTES", "78")
	t.Setenv("ATTACHMENT_MAX_FILES_PER_MESSAGE", "2")
	t.Setenv("ATTACHMENT_MAX_USER_STORED_BYTES", "789")
	t.Setenv("ATTACHMENT_MAX_USER_DAILY_BYTES", "321")
	t.Setenv("ATTACHMENT_MAX_TOTAL_BYTES", "654")
	t.Setenv("ATTACHMENT_RETENTION", "2h")
	t.Setenv("ATTACHMENT_STORAGE_DIR", "/var/lib/threaden/attachments")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load overridden config: %v", err)
	}
	if cfg.AttachmentMaxInputMediaBytes != 123 || cfg.AttachmentMaxArchiveBytes != 456 || cfg.AttachmentMaxOutputMediaBytes != 78 {
		t.Fatalf("unexpected size overrides: %#v", cfg)
	}
	if cfg.AttachmentMaxFilesPerMessage != 2 || cfg.AttachmentMaxUserStoredBytes != 789 || cfg.AttachmentMaxUserDailyBytes != 321 || cfg.AttachmentMaxTotalBytes != 654 {
		t.Fatalf("unexpected quota overrides: %#v", cfg)
	}
	if cfg.AttachmentRetention != 2*time.Hour || cfg.AttachmentStorageDir != "/var/lib/threaden/attachments" {
		t.Fatalf("unexpected path overrides: %s, %q", cfg.AttachmentRetention, cfg.AttachmentStorageDir)
	}
}

func TestLoadRejectsInvalidAttachmentLimits(t *testing.T) {
	for _, test := range []struct {
		name  string
		env   string
		value string
	}{
		{name: "media input", env: "ATTACHMENT_MAX_INPUT_MEDIA_BYTES", value: "0"},
		{name: "archive", env: "ATTACHMENT_MAX_ARCHIVE_BYTES", value: "-1"},
		{name: "file count", env: "ATTACHMENT_MAX_FILES_PER_MESSAGE", value: "0"},
		{name: "retention", env: "ATTACHMENT_RETENTION", value: "0s"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.env, test.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), test.env) {
				t.Fatalf("expected %s error, got %v", test.env, err)
			}
		})
	}
}
