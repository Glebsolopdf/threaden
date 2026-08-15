package attachments

import (
	"bytes"
	"testing"
)

func TestDetectsAudioContainers(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		file string
		mime string
	}{
		{name: "wav", data: []byte("RIFF\x18\x00\x00\x00WAVEfmt "), file: "voice.wav", mime: "audio/wav"},
		{name: "ogg", data: []byte("OggS\x00\x02\x00\x00"), file: "voice.ogg", mime: "audio/ogg"},
		{name: "mp3", data: []byte("ID3\x04\x00\x00\x00\x00\x00\x21"), file: "voice.mp3", mime: "audio/mpeg"},
		{name: "m4a", data: []byte("\x00\x00\x00\x18ftypM4A \x00\x00\x00\x00M4A "), file: "voice.m4a", mime: "audio/mp4"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kind, mime, err := Detect(bytes.NewReader(test.data), test.file)
			if err != nil || kind != string(KindAudio) || mime != test.mime {
				t.Fatalf("unexpected audio detection: kind=%q mime=%q err=%v", kind, mime, err)
			}
		})
	}
}

func TestDetectRejectsFakeAudioByExtension(t *testing.T) {
	kind, mime, err := Detect(bytes.NewReader([]byte("not audio")), "voice.mp3")
	if err == nil || kind != "" || mime != "" {
		t.Fatalf("expected fake audio rejection, got kind=%q mime=%q err=%v", kind, mime, err)
	}
}

func TestDetectsWebmAudioUsingContentTypeHint(t *testing.T) {
	data := append([]byte{0x1a, 0x45, 0xdf, 0xa3, 0x9f, 0x42, 0x86, 0x81}, []byte("webm")...)
	kind, mime, err := Detect(bytes.NewReader(data), "voice.webm", "audio/webm")
	if err != nil || kind != string(KindAudio) || mime != "audio/webm" {
		t.Fatalf("unexpected webm audio detection: kind=%q mime=%q err=%v", kind, mime, err)
	}
}

func TestDetectRejectsMismatchedImageExtension(t *testing.T) {
	kind, mime, err := Detect(bytes.NewReader([]byte("not a jpeg")), "photo.jpg")
	if err == nil || kind != "" || mime != "" {
		t.Fatalf("expected invalid image rejection, got kind=%q mime=%q err=%v", kind, mime, err)
	}
}

func TestSanitizeNameCannotCreatePath(t *testing.T) {
	name := SanitizeName("../../secret\\payload.zip")
	if name != "payload.zip" {
		t.Fatalf("unexpected sanitized name: %q", name)
	}
}

func TestDetectsPdfAsSafeFileRegardlessOfExtension(t *testing.T) {
	kind, mime, err := Detect(bytes.NewReader([]byte("%PDF-1.7\ncontent")), "document.bin")
	if err != nil || kind != string(KindFile) || mime != "application/pdf" {
		t.Fatalf("unexpected file detection: kind=%q mime=%q err=%v", kind, mime, err)
	}
}
