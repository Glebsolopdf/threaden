package account_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"voice-rooms/internal/attachments"
	"voice-rooms/internal/attachments/account"
	"voice-rooms/internal/model"
	"voice-rooms/internal/store/sqlite"
)

type deletedMessageEvent struct {
	groupID   string
	messageID string
}

type deletionPublisher struct {
	events []deletedMessageEvent
}

func (p *deletionPublisher) PublishMessageDeleted(_ context.Context, groupID, messageID string) error {
	p.events = append(p.events, deletedMessageEvent{groupID: groupID, messageID: messageID})
	return nil
}

func TestScheduleDeleteAllUsesFiveMinuteGracePeriod(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "account.db"))
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
	service := account.Service{DB: db, Limits: attachments.Limits{MaxUserStoredBytes: 50, MaxUserDailyBytes: 20, Retention: 72 * time.Hour}}
	now := time.Unix(100, 0).UTC()
	request, err := service.ScheduleDeleteAll(context.Background(), "u", now)
	if err != nil {
		t.Fatal(err)
	}
	if !request.ExecuteAt.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("execute_at=%s", request.ExecuteAt)
	}
	quota, err := service.Quotas(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if quota.PendingDelete == nil || quota.PendingDelete.ID != request.ID {
		t.Fatalf("pending request missing: %+v", quota.PendingDelete)
	}
}

func TestRunDueDeletesOnlyTheOwnersAttachmentMessages(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "cleanup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"u", "v"} {
		if _, err := db.Exec(`INSERT INTO users(id,email,display_name,avatar,password_hash,token_hash,created_at,last_seen_at) VALUES(?,?,?,'',X'',randomblob(32),1,1)`, user, user+"@example.com", user); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO groups(id,visibility,owner_id,name,avatar,invite_token,created_at,last_activity_at) VALUES('g','public','u','G','','invite',1,1)`); err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"u", "v"} {
		if _, err := db.Exec(`INSERT INTO group_members(group_id,user_id,joined_at,joined_at_nanos) VALUES('g',?,1,1000000000)`, user); err != nil {
			t.Fatal(err)
		}
	}
	for _, message := range []struct{ id, author string }{{"m1", "u"}, {"m2", "v"}} {
		if _, err := db.Exec(`INSERT INTO group_messages(id,group_id,author_id,body,created_at,kind,created_at_nanos,event) VALUES(?,?,?, 'caption',1,'chat',1000000000,'')`, message.id, "g", message.author); err != nil {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	paths := []string{filepath.Join(root, "u.bin"), filepath.Join(root, "v.bin")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("attachment"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Unix(100, 0).UTC()
	for _, item := range []model.Attachment{{ID: "a1", MessageID: "m1", GroupID: "g", OwnerID: "u", Kind: "file", Mime: "text/plain", Name: "u.txt", Size: 10, Path: paths[0], CreatedAt: now, ExpiresAt: now.Add(time.Hour)}, {ID: "a2", MessageID: "m2", GroupID: "g", OwnerID: "v", Kind: "file", Mime: "text/plain", Name: "v.txt", Size: 10, Path: paths[1], CreatedAt: now, ExpiresAt: now.Add(time.Hour)}} {
		if _, err := db.Exec(`INSERT INTO attachments(id,message_id,group_id,owner_id,kind,mime,name,size,path,created_at,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.MessageID, item.GroupID, item.OwnerID, item.Kind, item.Mime, item.Name, item.Size, item.Path, item.CreatedAt.Unix(), item.ExpiresAt.Unix()); err != nil {
			t.Fatal(err)
		}
	}
	publisher := &deletionPublisher{}
	service := account.Service{DB: db, Root: root, Publisher: publisher}
	if _, err := service.ScheduleDeleteAll(context.Background(), "u", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := service.RunDueDeletes(context.Background(), now, 10); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM group_messages WHERE id='m1'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("owner message remains: count=%d err=%v", count, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM group_messages WHERE id='m2'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("other user message changed: count=%d err=%v", count, err)
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatalf("owner file remains: %v", err)
	}
	if _, err := os.Stat(paths[1]); err != nil {
		t.Fatalf("other user file removed: %v", err)
	}
	if len(publisher.events) != 1 || publisher.events[0] != (deletedMessageEvent{groupID: "g", messageID: "m1"}) {
		t.Fatalf("deletion events = %+v", publisher.events)
	}
	if _, err := service.Quotas(context.Background(), "u"); err != nil {
		t.Fatal(err)
	}
}
