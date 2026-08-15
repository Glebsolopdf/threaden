package audio

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

func Duration(ctx context.Context, path, binary string) float64 {
	if binary == "" {
		binary = "ffprobe"
	}
	output, err := exec.CommandContext(ctx, binary, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}
