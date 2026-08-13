package attachments

import "time"

type Kind string

const (
	KindImage   Kind = "image"
	KindVideo   Kind = "video"
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
