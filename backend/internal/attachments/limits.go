package attachments

import "time"

const NewAccountGracePeriod = 24 * time.Hour

type Kind string

const (
	KindImage   Kind = "image"
	KindVideo   Kind = "video"
	KindAudio   Kind = "audio"
	KindFile    Kind = "file"
	KindArchive Kind = "archive"
)

type Limits struct {
	MaxInputMediaBytes  uint64
	MaxArchiveBytes     uint64
	MaxOutputMediaBytes uint64
	MaxFilesPerMessage  int
	MaxUserStoredBytes  uint64
	MaxUserDailyBytes   uint64
	MaxTotalBytes       uint64
	MinFreeBytes        uint64
	Retention           time.Duration
	StorageDir          string
}

func EffectiveLimits(base Limits, createdAt, now time.Time) Limits {
	if now.Before(createdAt.Add(NewAccountGracePeriod)) {
		base.MaxUserStoredBytes = minBytes(base.MaxUserStoredBytes, 20*1024*1024)
		base.MaxUserDailyBytes = 20 * 1024 * 1024
		return base
	}
	base.MaxUserDailyBytes = 0
	return base
}

func minBytes(left, right uint64) uint64 {
	if left < right {
		return left
	}
	return right
}
