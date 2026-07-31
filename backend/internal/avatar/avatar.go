package avatar

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/png"
)

const (
	MaxUploadBytes  = 8 << 20
	maxDataURLBytes = 700 << 10
	maxStoredBytes  = 96 << 10
	outputPixels    = 256
	maxDimension    = 4096
	maxSourcePixels = 4_000_000
)

func ValidStored(value string) bool {
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "data:image/") {
		return len(value) <= maxDataURLBytes
	}
	return ValidSymbol(value)
}

func ValidSymbol(value string) bool {
	count := utf8.RuneCountInString(value)
	if count < 1 || count > 8 || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func ProcessUpload(file multipart.File, header *multipart.FileHeader) (string, error) {
	if header.Size > MaxUploadBytes {
		return "", uploadError("avatar file is too large")
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxUploadBytes+1))
	if err != nil || len(data) == 0 {
		return "", uploadError("avatar file is invalid")
	}
	if len(data) > MaxUploadBytes {
		return "", uploadError("avatar file is too large")
	}
	mime := http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/gif" && mime != "image/webp" {
		return "", uploadError("avatar format must be jpeg, png, gif, or webp")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !safeDimensions(config.Width, config.Height) {
		return "", uploadError("avatar dimensions are invalid or too large")
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", uploadError("avatar file is invalid")
	}
	encoded, err := encode(squareResize(decoded, outputPixels))
	if err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(encoded), nil
}

func safeDimensions(width, height int) bool {
	if width < 1 || height < 1 || width > maxDimension || height > maxDimension {
		return false
	}
	return int64(width)*int64(height) <= maxSourcePixels
}

func squareResize(src image.Image, size int) *image.RGBA {
	bounds := src.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	x0 := bounds.Min.X + (bounds.Dx()-side)/2
	y0 := bounds.Min.Y + (bounds.Dy()-side)/2
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			sx := x0 + x*side/size
			sy := y0 + y*side/size
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func encode(img image.Image) ([]byte, error) {
	for quality := 82; quality >= 40; quality -= 7 {
		var out bytes.Buffer
		if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
		if out.Len() <= maxStoredBytes {
			return out.Bytes(), nil
		}
	}
	return nil, uploadError("avatar file is too large")
}

type uploadError string

func (e uploadError) Error() string { return string(e) }
