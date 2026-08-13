package video

import (
	"context"
	"testing"
)

func TestRunnerRejectsMissingBinary(t *testing.T) {
	runner := Runner{Binary: filepathDoesNotExist}
	if _, err := runner.Process(context.Background(), "input", "output", 1<<20); err == nil {
		t.Fatal("expected missing ffmpeg rejection")
	}
}

const filepathDoesNotExist = "/path/that/does/not/exist/ffmpeg"
