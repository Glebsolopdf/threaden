package attachments

import (
	"bytes"
	"testing"
)

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
