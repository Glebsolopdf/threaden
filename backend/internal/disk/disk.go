package disk

import (
	"fmt"
	"path/filepath"
	"syscall"
)

type Checker struct {
	path string
}

func NewChecker(path string) Checker {
	if path == "" || path == ":memory:" {
		path = "."
	}
	return Checker{path: filepath.Clean(path)}
}

func (c Checker) FreeBytes() (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(c.path, &stat); err != nil {
		return 0, fmt.Errorf("check free disk space for %s: %w", c.path, err)
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}
