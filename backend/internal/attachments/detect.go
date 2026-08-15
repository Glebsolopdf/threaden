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

func Detect(input io.Reader, originalName string, mimeHints ...string) (string, string, error) {
	reader := bufio.NewReader(input)
	header, err := reader.Peek(512)
	if err != nil && err != bufio.ErrBufferFull && err != io.EOF {
		return "", "", fmt.Errorf("read attachment header: %w", err)
	}
	if archive.Signature(header) {
		return string(KindArchive), archiveMime(header), nil
	}
	mimeType := http.DetectContentType(header)
	mimeHint := ""
	if len(mimeHints) > 0 {
		mimeHint = strings.ToLower(strings.TrimSpace(mimeHints[0]))
	}
	if strings.HasPrefix(mimeType, "image/") {
		return string(KindImage), mimeType, nil
	}
	if isAudio(header, mimeType, originalName, mimeHint) {
		return string(KindAudio), normalizeAudioMime(header, mimeType, originalName, mimeHint), nil
	}
	if isVideo(header, mimeType, mimeHint) {
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

func isAudio(header []byte, detected, originalName, hint string) bool {
	if strings.HasPrefix(hint, "audio/") && isWebM(header) {
		return true
	}
	if strings.HasPrefix(detected, "audio/") {
		return true
	}
	if len(header) >= 12 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WAVE" {
		return true
	}
	if len(header) >= 4 && string(header[:4]) == "OggS" {
		return true
	}
	if len(header) >= 3 && string(header[:3]) == "ID3" {
		return true
	}
	if len(header) >= 2 && header[0] == 0xff && header[1]&0xe0 == 0xe0 {
		return true
	}
	return isAudioMP4(header, originalName)
}

func isAudioMP4(header []byte, originalName string) bool {
	if len(header) < 12 || string(header[4:8]) != "ftyp" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	return ext == ".m4a" || ext == ".m4b" || ext == ".m4p" || string(header[8:12]) == "M4A "
}

func normalizeAudioMime(header []byte, detected, originalName, hint string) string {
	switch {
	case len(header) >= 12 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WAVE":
		return "audio/wav"
	case len(header) >= 4 && string(header[:4]) == "OggS":
		return "audio/ogg"
	case len(header) >= 3 && string(header[:3]) == "ID3":
		return "audio/mpeg"
	case isAudioMP4(header, originalName):
		return "audio/mp4"
	case strings.HasPrefix(hint, "audio/"):
		return hint
	case strings.HasPrefix(detected, "audio/"):
		return detected
	default:
		return "audio/webm"
	}
}

func isSafeFile(header []byte, originalName string) bool {
	if len(header) == 0 || bytes.Contains(header, []byte{0}) || !utf8.Valid(header) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	if strings.Contains(".jpg.jpeg.png.gif.webp.bmp.svg.mp4.webm.mov.m4a.m4b.m4p.mp3.wav.ogg.opus.zip.7z.rar.tar.gz.tgz", ext) {
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

func isVideo(header []byte, detected, hint string) bool {
	if strings.HasPrefix(hint, "video/") && isWebM(header) {
		return true
	}
	if detected == "video/webm" || detected == "video/mp4" {
		return true
	}
	return len(header) >= 12 && string(header[4:8]) == "ftyp"
}

func isWebM(header []byte) bool {
	return len(header) >= 4 && header[0] == 0x1a && header[1] == 0x45 && header[2] == 0xdf && header[3] == 0xa3
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
