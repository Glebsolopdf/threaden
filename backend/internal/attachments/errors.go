package attachments

import "errors"

var (
	ErrUnsupportedFormat = errors.New("unsupported attachment format")
	ErrInvalidFormat     = errors.New("invalid attachment format")
	ErrTooLarge          = errors.New("attachment is too large")
	ErrTooMany           = errors.New("too many attachments")
	ErrQuotaExceeded     = errors.New("attachment quota exceeded")
	ErrLowDisk           = errors.New("not enough disk space for attachment")
	ErrProcessing        = errors.New("attachment processing failed")
)
