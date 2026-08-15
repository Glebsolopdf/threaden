package image

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestProcessReencodesPngAsJpeg(t *testing.T) {
	var source bytes.Buffer
	input := image.NewRGBA(image.Rect(0, 0, 2, 2))
	input.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&source, input); err != nil {
		t.Fatal(err)
	}
	result, err := Process(bytes.NewReader(source.Bytes()), 1<<20, 100)
	if err != nil || result.Mime != "image/jpeg" || result.Size <= 0 {
		t.Fatalf("unexpected result=%+v err=%v", result, err)
	}
}

func TestProcessCompressesLargePhotoUnderOutputLimit(t *testing.T) {
	photo := image.NewRGBA(image.Rect(0, 0, 2000, 1500))
	seed := uint32(1)
	for y := 0; y < 1500; y++ {
		for x := 0; x < 2000; x++ {
			seed = seed*1664525 + 1013904223
			photo.SetRGBA(x, y, color.RGBA{R: uint8(seed), G: uint8(seed >> 8), B: uint8(seed >> 16), A: 255})
		}
	}
	var source bytes.Buffer
	if err := jpeg.Encode(&source, photo, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}

	result, err := Process(bytes.NewReader(source.Bytes()), 1<<20, 16_000_000)
	if err != nil || result.Size <= 0 || result.Size > 1<<20 {
		t.Fatalf("large photo was not compressed: result=%+v err=%v", result, err)
	}
}
