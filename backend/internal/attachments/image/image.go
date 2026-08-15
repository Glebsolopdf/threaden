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

	"golang.org/x/image/draw"
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
	encoded, err := encodeUnderLimit(decoded, maxOutput)
	if err != nil {
		return Result{}, err
	}
	file, err := os.CreateTemp("", "threaden-image-*")
	if err != nil {
		return Result{}, err
	}
	path := file.Name()
	defer func() { _ = file.Close() }()
	if _, err := file.Write(encoded); err != nil {
		_ = os.Remove(path)
		return Result{}, fmt.Errorf("write encoded image: %w", err)
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

func encodeUnderLimit(source image.Image, maxOutput uint64) ([]byte, error) {
	for maxDimension := 2048; maxDimension >= 320; maxDimension = maxDimension * 3 / 4 {
		candidate := resize(source, maxDimension)
		for quality := 75; quality >= 30; quality -= 15 {
			var encoded bytes.Buffer
			if err := jpeg.Encode(&encoded, candidate, &jpeg.Options{Quality: quality}); err != nil {
				return nil, fmt.Errorf("encode image: %w", err)
			}
			if uint64(encoded.Len()) <= maxOutput {
				return encoded.Bytes(), nil
			}
		}
	}
	return nil, fmt.Errorf("encoded image exceeds limit")
}

func resize(source image.Image, maxDimension int) image.Image {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= maxDimension && height <= maxDimension {
		return source
	}
	if width >= height {
		height = height * maxDimension / width
		width = maxDimension
	} else {
		width = width * maxDimension / height
		height = maxDimension
	}
	destination := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(destination, destination.Bounds(), source, bounds, draw.Over, nil)
	return destination
}

func ProcessFile(path string, maxOutput uint64) (Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer file.Close()
	return Process(file, maxOutput, maxPixels)
}
