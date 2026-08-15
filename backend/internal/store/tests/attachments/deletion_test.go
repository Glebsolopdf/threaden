package attachments_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	attachmentstore "voice-rooms/internal/store/attachments"
	"voice-rooms/internal/store/sqlite"
)

func TestAttachmentDeleteRequestIsIdempotentAndCancelable(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "deletion.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(id,email,display_name,avatar,password_hash,token_hash,created_at,last_seen_at) VALUES('u','u@example.com','U','',X'',zeroblob(32),1,1)`); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0).UTC()
	first, err := attachmentstore.CreateDeleteRequest(context.Background(), db, "u", now, now.Add(30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	second, err := attachmentstore.CreateDeleteRequest(context.Background(), db, "u", now.Add(time.Minute), now.Add(31*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || !second.ExecuteAt.Equal(first.ExecuteAt) {
		t.Fatalf("duplicate request was created: first=%+v second=%+v", first, second)
	}
	if err := attachmentstore.CancelDeleteRequest(context.Background(), db, "u"); err != nil {
		t.Fatal(err)
	}
	if _, err := attachmentstore.GetDeleteRequest(context.Background(), db, "u"); err == nil {
		t.Fatal("pending deletion remains after cancellation")
	}
}
