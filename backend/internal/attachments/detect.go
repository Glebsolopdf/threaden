package attachments

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"voice-rooms/internal/attachments/archive"
)

func Detect(input io.Reader, originalName string) (string, string, error) {
	reader := bufio.NewReader(input)
	header, err := reader.Peek(512)
	if err != nil && err != bufio.ErrBufferFull && err != io.EOF {
		return "", "", fmt.Errorf("read attachment header: %w", err)
	}
	if archive.Signature(header) {
		return string(KindArchive), archiveMime(header), nil
	}
	mimeType := http.DetectContentType(header)
	if strings.HasPrefix(mimeType, "image/") {
		return string(KindImage), mimeType, nil
	}
	if isVideo(header, mimeType) {
		return string(KindVideo), normalizeVideoMime(header, mimeType), nil
	}
	if isSafeFile(header, originalName) {
		if len(header) >= 5 && string(header[:5]) == "%PDF-" {
			return string(KindFile), "application/pdf", nil
		}
		return string(KindFile), "text/plain; charset=utf-8", nil
	}
	_ = originalName
	return "", "", ErrUnsupportedFormat
}

func isSafeFile(header []byte, originalName string) bool {
	if len(header) == 0 || bytes.Contains(header, []byte{0}) || !utf8.Valid(header) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	if strings.Contains(".jpg.jpeg.png.gif.webp.bmp.svg.mp4.webm.mov.zip.7z.rar.tar.gz.tgz", ext) {
		return false
	}
	return len(header) >= 5 && string(header[:5]) == "%PDF-" || utf8.Valid(header)
}

func archiveMime(header []byte) string {
	if len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		return "application/gzip"
	}
	return "application/zip"
}

func isVideo(header []byte, detected string) bool {
	if detected == "video/webm" || detected == "video/mp4" {
		return true
	}
	return len(header) >= 12 && string(header[4:8]) == "ftyp"
}

func normalizeVideoMime(header []byte, detected string) string {
	if detected == "video/webm" {
		return detected
	}
	if len(header) >= 12 && string(header[4:8]) == "ftyp" {
		return "video/mp4"
	}
	return mime.TypeByExtension(filepath.Ext("video.webm"))
}

func SanitizeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "attachment"
	}
	return name
}
