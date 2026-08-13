package image

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
)

type Result struct {
	Path string
	Mime string
	Size int64
}

const maxPixels = 16_000_000

func Process(input io.Reader, maxOutput uint64, maxPixelsAllowed int64) (Result, error) {
	if maxPixelsAllowed <= 0 || maxPixelsAllowed > maxPixels {
		maxPixelsAllowed = maxPixels
	}
	data, err := io.ReadAll(io.LimitReader(input, 10*1024*1024+1))
	if err != nil {
		return Result{}, err
	}
	if len(data) > 10*1024*1024 {
		return Result{}, fmt.Errorf("image input exceeds limit")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || int64(config.Width)*int64(config.Height) > maxPixelsAllowed {
		return Result{}, fmt.Errorf("invalid or oversized image")
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Result{}, fmt.Errorf("decode image: %w", err)
	}
	file, err := os.CreateTemp("", "threaden-image-*")
	if err != nil {
		return Result{}, err
	}
	path := file.Name()
	defer func() { _ = file.Close() }()
	if err := jpeg.Encode(file, decoded, &jpeg.Options{Quality: 75}); err != nil {
		_ = os.Remove(path)
		return Result{}, fmt.Errorf("encode image: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return Result{}, err
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > int64(maxOutput) {
		_ = os.Remove(path)
		return Result{}, fmt.Errorf("encoded image exceeds limit")
	}
	return Result{Path: path, Mime: "image/jpeg", Size: info.Size()}, nil
}

func ProcessFile(path string, maxOutput uint64) (Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer file.Close()
	return Process(file, maxOutput, maxPixels)
}
