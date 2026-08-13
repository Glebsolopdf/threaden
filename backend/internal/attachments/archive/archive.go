package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	MaxEntries       = 10000
	MaxExpandedBytes = 100 * 1024 * 1024
)

func Signature(header []byte) bool {
	return len(header) >= 4 && ((header[0] == 'P' && header[1] == 'K' && header[2] == 3 && header[3] == 4) ||
		(len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b) ||
		(len(header) >= 265 && string(header[257:262]) == "ustar"))
}

func Validate(path string, maxBytes int64, maxExpanded int64) error {
	if maxExpanded <= 0 || maxExpanded > MaxExpandedBytes {
		maxExpanded = MaxExpandedBytes
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat archive: %w", err)
	}
	if info.Size() <= 0 || info.Size() > maxBytes {
		return fmt.Errorf("archive exceeds input limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()
	header := make([]byte, 512)
	n, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return fmt.Errorf("read archive header: %w", err)
	}
	header = header[:n]
	if !Signature(header) {
		return fmt.Errorf("unknown archive signature")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if len(header) >= 4 && header[0] == 'P' && header[1] == 'K' {
		return validateZip(file, maxExpanded)
	}
	if len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		return validateGzip(file, maxExpanded)
	}
	return validateTar(file, maxExpanded)
}

func validateZip(file *os.File, maxExpanded int64) error {
	reader, err := zip.NewReader(file, fileSize(file))
	if err != nil {
		return fmt.Errorf("read zip: %w", err)
	}
	if len(reader.File) > MaxEntries {
		return fmt.Errorf("archive has too many entries")
	}
	var expanded int64
	for _, entry := range reader.File {
		if unsafeName(entry.Name) || entry.Mode()&os.ModeSymlink != 0 || entry.Mode()&os.ModeType != 0 && !entry.FileInfo().IsDir() {
			return fmt.Errorf("archive contains unsafe entry")
		}
		if entry.UncompressedSize64 > uint64(maxExpanded) || expanded > maxExpanded-int64(entry.UncompressedSize64) {
			return fmt.Errorf("archive expands beyond limit")
		}
		expanded += int64(entry.UncompressedSize64)
	}
	return nil
}

func validateGzip(file *os.File, maxExpanded int64) error {
	reader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read gzip: %w", err)
	}
	defer reader.Close()
	count, err := io.Copy(io.Discard, io.LimitReader(reader, maxExpanded+1))
	if err != nil {
		return fmt.Errorf("read gzip payload: %w", err)
	}
	if count > maxExpanded {
		return fmt.Errorf("archive expands beyond limit")
	}
	return nil
}

func validateTar(file *os.File, maxExpanded int64) error {
	reader := tar.NewReader(file)
	var expanded int64
	for entries := 0; entries < MaxEntries; entries++ {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if unsafeName(header.Name) || header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink || header.Typeflag == tar.TypeChar || header.Typeflag == tar.TypeBlock || header.Typeflag == tar.TypeFifo {
			return fmt.Errorf("archive contains unsafe entry")
		}
		if header.Size < 0 || header.Size > maxExpanded || expanded > maxExpanded-header.Size {
			return fmt.Errorf("archive expands beyond limit")
		}
		expanded += header.Size
		if _, err := io.Copy(io.Discard, reader); err != nil {
			return fmt.Errorf("read tar payload: %w", err)
		}
	}
	return fmt.Errorf("archive has too many entries")
}

func unsafeName(name string) bool {
	clean := filepath.Clean(filepath.ToSlash(name))
	return name == "" || strings.HasPrefix(name, "/") || clean == ".." || strings.HasPrefix(clean, "../")
}

func fileSize(file *os.File) int64 {
	info, err := file.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}
