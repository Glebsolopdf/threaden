package attachments_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"voice-rooms/internal/model"
	attachmentstore "voice-rooms/internal/store/attachments"
	"voice-rooms/internal/store/schema"
	"voice-rooms/internal/store/sqlite"
)

func TestMetadataRoundTripAndQuotas(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "attachments.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if schema.LatestVersion != 23 {
		t.Fatalf("attachment migration is not latest: %d", schema.LatestVersion)
	}
	_, err = db.Exec(`INSERT INTO users(id,email,display_name,avatar,password_hash,token_hash,created_at,last_seen_at) VALUES('u','u@example.com','U','',X'',zeroblob(32),1,1)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO groups(id,visibility,owner_id,name,avatar,invite_token,created_at,last_activity_at) VALUES('g','public','u','G','','invite',1,1)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO group_members(group_id,user_id,joined_at,joined_at_nanos) VALUES('g','u',1,1000000000)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO group_messages(id,group_id,author_id,body,created_at,kind,created_at_nanos,event) VALUES('m','g','u','',1,'chat',1000000000,'')`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0).UTC()
	item := model.Attachment{ID: "a", MessageID: "m", GroupID: "g", OwnerID: "u", Kind: "archive", Mime: "application/zip", Name: "backup.any", Size: 7, Path: "a/1", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := attachmentstore.Add(context.Background(), db, item); err != nil {
		t.Fatal(err)
	}
	got, err := attachmentstore.ListForMessage(context.Background(), db, "m")
	if err != nil || len(got) != 1 || got[0].Path != item.Path {
		t.Fatalf("metadata round trip: %+v, %v", got, err)
	}
	if total, err := attachmentstore.SumForOwner(context.Background(), db, "u"); err != nil || total != item.Size {
		t.Fatalf("owner quota sum=%d err=%v", total, err)
	}
	if total, err := attachmentstore.SumAll(context.Background(), db); err != nil || total != item.Size {
		t.Fatalf("global quota sum=%d err=%v", total, err)
	}
}
