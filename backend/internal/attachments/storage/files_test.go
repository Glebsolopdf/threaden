package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMoveFileCopiesAcrossFileSystems(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cross-device test uses the Linux shared-memory filesystem")
	}
	if _, err := os.Stat("/dev/shm"); err != nil {
		t.Skip("shared-memory filesystem is unavailable")
	}
	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, "processed-image")
	if err := os.WriteFile(source, []byte("processed"), 0o600); err != nil {
		t.Fatal(err)
	}
	destinationDir, err := os.MkdirTemp("/dev/shm", "threaden-storage-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(destinationDir) })
	destination := filepath.Join(destinationDir, "attachment")

	if err := moveFile(source, destination); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(destination)
	if err != nil || string(content) != "processed" {
		t.Fatalf("destination content = %q, err = %v", string(content), err)
	}
	if _, err := os.Stat(source); err == nil || !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("source should be removed, got %v", err)
	}
}
