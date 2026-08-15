package audio_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"voice-rooms/internal/attachments/audio"
)

func TestDurationReadsWaveMetadata(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is required to generate the audio fixture")
	}
	path := filepath.Join(t.TempDir(), "voice.wav")
	if err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "anullsrc=r=8000:cl=mono", "-t", "1", "-y", path).Run(); err != nil {
		t.Fatal(err)
	}

	if got := audio.Duration(context.Background(), path, "ffprobe"); got < 0.99 || got > 1.01 {
		t.Fatalf("duration=%v, want about 1 second", got)
	}
}
