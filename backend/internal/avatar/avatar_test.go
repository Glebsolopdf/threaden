package avatar

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"testing"
)

type memoryFile struct{ *bytes.Reader }

func (memoryFile) Close() error { return nil }

func TestProcessUploadRejectsDeclaredImageBomb(t *testing.T) {
	data := pngHeader(100_000, 100_000)
	_, err := ProcessUpload(memoryFile{bytes.NewReader(data)}, &multipart.FileHeader{Size: int64(len(data))})
	if err == nil || err.Error() != "avatar dimensions are invalid or too large" {
		t.Fatalf("expected dimension rejection, got %v", err)
	}
}

func TestProcessUploadAcceptsSmallPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 5), B: 80, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	value, err := ProcessUpload(memoryFile{bytes.NewReader(encoded.Bytes())}, &multipart.FileHeader{Size: int64(encoded.Len())})
	if err != nil {
		t.Fatal(err)
	}
	if len(value) < len("data:image/jpeg;base64,") || value[:len("data:image/jpeg;base64,")] != "data:image/jpeg;base64," {
		t.Fatalf("unexpected stored avatar: %q", value)
	}
}

func pngHeader(width, height uint32) []byte {
	var out bytes.Buffer
	out.Write([]byte("\x89PNG\r\n\x1a\n"))
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], width)
	binary.BigEndian.PutUint32(ihdr[4:8], height)
	ihdr[8], ihdr[9], ihdr[10], ihdr[11], ihdr[12] = 8, 6, 0, 0, 0
	writeChunk(&out, "IHDR", ihdr)
	writeChunk(&out, "IEND", nil)
	return out.Bytes()
}

func writeChunk(out *bytes.Buffer, kind string, data []byte) {
	_ = binary.Write(out, binary.BigEndian, uint32(len(data)))
	out.WriteString(kind)
	out.Write(data)
	checksum := crc32.NewIEEE()
	_, _ = checksum.Write([]byte(kind))
	_, _ = checksum.Write(data)
	_ = binary.Write(out, binary.BigEndian, checksum.Sum32())
}
