package image

import (
	"bytes"
	"image"
	"image/color"
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
