package abuse

import "time"

type Limit struct {
	Capacity int
	Refill   time.Duration
}

type Config struct {
	TrustedProxies       []string
	GlobalLimit          Limit
	RegisterLimit        Limit
	LoginLimit           Limit
	GroupCreateLimit     Limit
	ProfileUpdateLimit   Limit
	GroupJoinLimit       Limit
	MessageLimit         Limit
	SearchLimit          Limit
	HeavyLimit           Limit
	MaxMessageRunes      int
	MaxLinks             int
	MaxMentions          int
	MinMessageInterval   time.Duration
	NewAccountAge        time.Duration
	NewAccountMessageCap int
	IdempotencyTTL       time.Duration
	BucketTTL            time.Duration
}

func DefaultConfig() Config {
	return Config{
		GlobalLimit:          Limit{Capacity: 900, Refill: time.Minute},
		RegisterLimit:        Limit{Capacity: 30, Refill: time.Hour},
		LoginLimit:           Limit{Capacity: 12, Refill: 15 * time.Minute},
		GroupCreateLimit:     Limit{Capacity: 1, Refill: 3 * time.Minute},
		ProfileUpdateLimit:   Limit{Capacity: 1, Refill: 3 * time.Minute},
		GroupJoinLimit:       Limit{Capacity: 60, Refill: time.Hour},
		MessageLimit:         Limit{Capacity: 20, Refill: time.Minute},
		SearchLimit:          Limit{Capacity: 60, Refill: time.Minute},
		HeavyLimit:           Limit{Capacity: 30, Refill: time.Minute},
		MaxMessageRunes:      2000,
		MaxLinks:             4,
		MaxMentions:          12,
		MinMessageInterval:   900 * time.Millisecond,
		NewAccountAge:        24 * time.Hour,
		NewAccountMessageCap: 8,
		IdempotencyTTL:       10 * time.Minute,
		BucketTTL:            6 * time.Hour,
	}
}

func RefillPerSecond(limit Limit) float64 {
	if limit.Capacity <= 0 || limit.Refill <= 0 {
		return 1
	}
	return float64(limit.Capacity) / limit.Refill.Seconds()
}
