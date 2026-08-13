package video

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Runner struct {
	Binary string
}

func (r Runner) Process(ctx context.Context, input, output string, maxOutput int64) (int64, error) {
	if r.Binary == "" {
		r.Binary = "ffmpeg"
	}
	command := exec.CommandContext(ctx, r.Binary, "-nostdin", "-hide_banner", "-loglevel", "error", "-i", input,
		"-map_metadata", "-1", "-c:v", "libx264", "-preset", "veryfast", "-crf", "35", "-c:a", "aac", "-b:a", "48k", "-movflags", "+faststart", "-y", output)
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("ffmpeg process video: %w", err)
	}
	info, err := os.Stat(output)
	if err != nil {
		return 0, fmt.Errorf("stat transcoded video: %w", err)
	}
	if info.Size() <= 0 || info.Size() > maxOutput {
		return 0, fmt.Errorf("transcoded video exceeds limit")
	}
	return info.Size(), nil
}

func NewTempOutput(dir string) (string, error) {
	file, err := os.CreateTemp(dir, "video-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return filepath.Clean(path), nil
}
