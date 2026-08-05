package store_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"voice-rooms/internal/model"
	store "voice-rooms/internal/store"
)

func TestMessageReadReceipts(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "reads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	author := model.User{ID: "usr_read_author", Email: "read-author@example.com", DisplayName: "Author", CreatedAt: now}
	reader := model.User{ID: "usr_read_reader", Email: "read-reader@example.com", DisplayName: "Reader", CreatedAt: now}
	for _, user := range []model.User{author, reader} {
		if err := st.CreateUser(ctx, user, []byte("hash"), sha256.Sum256([]byte(user.ID))); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreateGroup(ctx, store.NewGroup{ID: "grp_reads", Visibility: "public", OwnerID: author.ID, Name: "Reads", InviteToken: "inv_reads"}, now, 0); err != nil {
		t.Fatal(err)
	}
	if err := st.JoinGroup(ctx, "grp_reads", reader.ID, now); err != nil {
		t.Fatal(err)
	}
	message := model.GroupMessage{ID: "msg_reads", GroupID: "grp_reads", Author: author, Body: "hello", CreatedAt: now}
	if err := st.AddMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	messages, err := st.Messages(ctx, "grp_reads", now.Add(-time.Minute), 10, author.ID)
	if err != nil || len(messages) != 1 || messages[0].Read {
		t.Fatalf("message should be unread: %+v, %v", messages, err)
	}
	receipts, err := st.MarkGroupMessagesRead(ctx, "grp_reads", reader.ID, message.ID, now.Add(time.Second))
	if err != nil || len(receipts) != 1 || receipts[0].MessageID != message.ID {
		t.Fatalf("read receipt: %+v, %v", receipts, err)
	}
	receipts, err = st.MarkGroupMessagesRead(ctx, "grp_reads", reader.ID, message.ID, now.Add(time.Second))
	if err != nil || len(receipts) != 0 {
		t.Fatalf("second mark should emit no receipts: %+v, %v", receipts, err)
	}
	messages, err = st.Messages(ctx, "grp_reads", now.Add(-time.Minute), 10, author.ID)
	if err != nil || len(messages) != 1 || !messages[0].Read {
		t.Fatalf("message should be read: %+v, %v", messages, err)
	}
}

func TestAddMessageReplyToMissingTarget(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "fkreply.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	author := model.User{ID: "usr_fk_author", Email: "fk@example.com", DisplayName: "FK", CreatedAt: now}
	if err := st.CreateUser(ctx, author, []byte("hash"), sha256.Sum256([]byte(author.ID))); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateGroup(ctx, store.NewGroup{ID: "grp_fk", Visibility: "public", OwnerID: author.ID, Name: "FK", InviteToken: "inv_fk"}, now, 0); err != nil {
		t.Fatal(err)
	}
	err = st.AddMessage(ctx, model.GroupMessage{ID: "msg_fk", GroupID: "grp_fk", Author: author, Body: "hello", CreatedAt: now, ReplyTo: &model.MessageReference{ID: "msg_ghost", Author: author, Body: "x"}})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("reply to missing target: expected ErrNotFound, got %v", err)
	}
}
