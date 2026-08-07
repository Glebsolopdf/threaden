package cleanup_test

import (
	"context"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	"voice-rooms/internal/model"
	store "voice-rooms/internal/store"
)

func TestScheduledGroupActivityCancelsDeletion(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "scheduled-group-activity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	owner := model.User{ID: "usr_reactivate", Email: "reactivate@example.com", DisplayName: "Reactivate", CreatedAt: now}
	if err := st.CreateUser(ctx, owner, []byte("hash"), sha256.Sum256([]byte("reactivate"))); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateGroup(ctx, store.NewGroup{ID: "grp_reactivate", Visibility: "public", OwnerID: owner.ID, Name: "Reactivate", InviteToken: "inv_reactivate"}, now.Add(-8*24*time.Hour), 0); err != nil {
		t.Fatal(err)
	}
	if count, err := st.ScheduleInactiveGroups(ctx, now.Add(-7*24*time.Hour), now.Add(time.Hour), 10); err != nil || count != 1 {
		t.Fatalf("schedule count=%d err=%v", count, err)
	}
	if err := st.AddMessage(ctx, model.GroupMessage{ID: "msg_reactivate", GroupID: "grp_reactivate", Author: owner, Body: "active again", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	deleted, err := st.DeleteScheduledGroups(ctx, now.Add(2*time.Hour), now.Add(-7*24*time.Hour), 10)
	if err != nil || len(deleted) != 0 {
		t.Fatalf("active group deleted: %+v err=%v", deleted, err)
	}
	if _, err := st.Group(ctx, "grp_reactivate"); err != nil {
		t.Fatalf("reactivated group missing: %v", err)
	}
}
