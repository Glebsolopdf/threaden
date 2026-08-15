package attachments_test

import (
	"bytes"
	"os"
	"testing"

	"voice-rooms/internal/attachments"
)

func TestProcessorProcessesAudio(t *testing.T) {
	source := append([]byte("RIFF\x18\x00\x00\x00WAVEfmt "), []byte("audio data")...)
	result, err := (attachments.Processor{MaxInputMedia: 1 << 20, MaxArchive: 1 << 20}).Process(
		t.Context(), bytes.NewReader(source), "voice.wav", int64(len(source)),
	)
	if err != nil {
		t.Fatalf("process audio: %v", err)
	}
	defer os.Remove(result.Path)
	if result.Kind != attachments.KindAudio || result.Mime != "audio/wav" || result.Size != int64(len(source)) {
		t.Fatalf("unexpected processed audio: %+v", result)
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("processed audio is not stored: %v", err)
	}
}
