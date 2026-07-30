package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr             string
	DatabasePath         string
	LiveKitURL           string
	LiveKitPublicURL     string
	LiveKitAPIKey        string
	LiveKitAPISecret     string
	RoomTTL              time.Duration
	LiveKitTokenTTL      time.Duration
	SessionTTL           time.Duration
	SessionIdleTTL       time.Duration
	SessionCookieSecure  bool
	MaxRoomParticipants  int
	CORSAllowedOrigins   []string
	TrustedProxies       []string
	RateLimitBucketTTL   time.Duration
	MaxMessageRunes      int
	MaxMessageLinks      int
	MaxMessageMentions   int
	InactiveGroupTTL     time.Duration
	GroupDeleteGrace     time.Duration
	GroupCleanupInterval time.Duration
	GroupCleanupBatch    int
	GroupCleanupDryRun   bool
	LowDiskMinFreeBytes  uint64
	LowDiskCheckInterval time.Duration
	LowDiskCleanupBatch  int
	LowDiskMessageMinAge time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:         getenv("HTTP_ADDR", ":8080"),
		DatabasePath:     getenv("DATABASE_PATH", "./data/app.db"),
		LiveKitURL:       getenv("LIVEKIT_URL", "ws://livekit:7880"),
		LiveKitPublicURL: getenv("LIVEKIT_PUBLIC_URL", "ws://localhost:7880"),
		LiveKitAPIKey:    getenv("LIVEKIT_API_KEY", "devkey"),
		LiveKitAPISecret: getenv("LIVEKIT_API_SECRET", "secret"),
	}
	var err error
	if cfg.RoomTTL, err = duration("ROOM_TTL", "12h"); err != nil {
		return Config{}, err
	}
	if cfg.LiveKitTokenTTL, err = duration("LIVEKIT_TOKEN_TTL", "30m"); err != nil {
		return Config{}, err
	}
	if cfg.SessionTTL, err = duration("SESSION_TTL", "168h"); err != nil {
		return Config{}, err
	}
	if cfg.SessionIdleTTL, err = duration("SESSION_IDLE_TTL", "24h"); err != nil {
		return Config{}, err
	}
	if cfg.SessionIdleTTL > cfg.SessionTTL {
		return Config{}, fmt.Errorf("SESSION_IDLE_TTL must not exceed SESSION_TTL")
	}
	max, err := strconv.Atoi(getenv("MAX_ROOM_PARTICIPANTS", "16"))
	if err != nil || max < 1 {
		return Config{}, fmt.Errorf("MAX_ROOM_PARTICIPANTS must be a positive integer")
	}
	cfg.MaxRoomParticipants = max
	if cfg.RateLimitBucketTTL, err = duration("RATE_LIMIT_BUCKET_TTL", "6h"); err != nil {
		return Config{}, err
	}
	if cfg.InactiveGroupTTL, err = duration("INACTIVE_GROUP_TTL", "168h"); err != nil {
		return Config{}, err
	}
	if cfg.GroupDeleteGrace, err = duration("GROUP_DELETE_GRACE", "24h"); err != nil {
		return Config{}, err
	}
	if cfg.GroupCleanupInterval, err = duration("GROUP_CLEANUP_INTERVAL", "1h"); err != nil {
		return Config{}, err
	}
	if cfg.LowDiskCheckInterval, err = duration("LOW_DISK_CHECK_INTERVAL", "5m"); err != nil {
		return Config{}, err
	}
	if cfg.LowDiskMessageMinAge, err = duration("LOW_DISK_MESSAGE_MIN_AGE", "24h"); err != nil {
		return Config{}, err
	}
	if cfg.MaxMessageRunes, err = positiveInt("MAX_MESSAGE_RUNES", 2000); err != nil {
		return Config{}, err
	}
	if cfg.MaxMessageLinks, err = positiveInt("MAX_MESSAGE_LINKS", 4); err != nil {
		return Config{}, err
	}
	if cfg.MaxMessageMentions, err = positiveInt("MAX_MESSAGE_MENTIONS", 12); err != nil {
		return Config{}, err
	}
	if cfg.GroupCleanupBatch, err = positiveInt("GROUP_CLEANUP_BATCH", 20); err != nil {
		return Config{}, err
	}
	if cfg.LowDiskCleanupBatch, err = positiveInt("LOW_DISK_CLEANUP_BATCH", 500); err != nil {
		return Config{}, err
	}
	if cfg.LowDiskMinFreeBytes, err = bytesValue("LOW_DISK_MIN_FREE_BYTES", 5*1024*1024*1024); err != nil {
		return Config{}, err
	}
	cfg.GroupCleanupDryRun = boolValue("GROUP_CLEANUP_DRY_RUN", true)
	cfg.SessionCookieSecure = boolValue("SESSION_COOKIE_SECURE", false)
	for _, origin := range strings.Split(getenv("CORS_ALLOWED_ORIGINS", "*"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, origin)
		}
	}
	for _, proxy := range strings.Split(getenv("TRUSTED_PROXIES", "127.0.0.1,::1"), ",") {
		if proxy = strings.TrimSpace(proxy); proxy != "" {
			cfg.TrustedProxies = append(cfg.TrustedProxies, proxy)
		}
	}
	if cfg.HTTPAddr == "" || cfg.DatabasePath == "" || cfg.LiveKitURL == "" ||
		cfg.LiveKitPublicURL == "" || cfg.LiveKitAPIKey == "" || cfg.LiveKitAPISecret == "" {
		return Config{}, fmt.Errorf("HTTP, database, and LiveKit settings must not be empty")
	}
	if len(cfg.CORSAllowedOrigins) == 0 {
		return Config{}, fmt.Errorf("CORS_ALLOWED_ORIGINS must contain at least one origin")
	}
	return cfg, nil
}

func duration(name, fallback string) (time.Duration, error) {
	value := getenv(name, fallback)
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return d, nil
}

func positiveInt(name string, fallback int) (int, error) {
	value, err := strconv.Atoi(getenv(name, strconv.Itoa(fallback)))
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func bytesValue(name string, fallback uint64) (uint64, error) {
	value, err := strconv.ParseUint(getenv(name, strconv.FormatUint(fallback, 10)), 10, 64)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer number of bytes", name)
	}
	return value, nil
}

func boolValue(name string, fallback bool) bool {
	value := strings.ToLower(getenv(name, strconv.FormatBool(fallback)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func getenv(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return strings.TrimSpace(value)
	}
	return fallback
}
