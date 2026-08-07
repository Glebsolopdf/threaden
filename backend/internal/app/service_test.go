package app

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"voice-rooms/internal/model"
	"voice-rooms/internal/store"
)

type fakeVoice struct {
	ttl     time.Duration
	deleted []string
}

func (f *fakeVoice) PublicURL() string { return "ws://voice.test" }
func (f *fakeVoice) JoinToken(_ string, _ model.User, ttl time.Duration) (string, error) {
	f.ttl = ttl
	return "jwt", nil
}
func (f *fakeVoice) DeleteRoom(_ context.Context, code string) error {
	f.deleted = append(f.deleted, code)
	return nil
}
func (*fakeVoice) RemoveParticipant(context.Context, string, string) error { return nil }

func TestRoomCodeAndEffectiveTokenTTL(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	voice := &fakeVoice{}
	service := New(st, voice, time.Hour, 30*time.Minute, 2, slog.Default())
	now := time.Unix(1_700_000_000, 0).UTC()
	service.now = func() time.Time { return now }
	service.random = bytes.NewReader(bytes.Repeat([]byte{0}, 48))

	created, err := service.Register(context.Background(), "gleb@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	service.random = bytes.NewReader(bytes.Repeat([]byte{0}, 2626))
	room, err := service.CreateRoom(context.Background(), created.User)
	if err != nil {
		t.Fatal(err)
	}
	if room.Code != "AAAAAAAAAAAAAAAAAAAAAAAAAA" {
		t.Fatalf("unexpected deterministic room code %q", room.Code)
	}
	if _, err := service.CreateRoom(context.Background(), created.User); !Is(err, ErrRoomCodeUnavailable) {
		t.Fatalf("expected collision exhaustion, got %v", err)
	}
	if _, err := service.JoinRoom(context.Background(), room.Code, created.User); err != nil {
		t.Fatal(err)
	}
	if voice.ttl != 30*time.Minute {
		t.Fatalf("token TTL must use configured cap, got %s", voice.ttl)
	}
}

func TestRegisterLoginAndProfile(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "users.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	service := New(st, &fakeVoice{}, time.Hour, time.Minute, 2, slog.Default())
	service.random = bytes.NewReader(bytes.Repeat([]byte{2}, 96))

	created, err := service.Register(context.Background(), "User@Example.COM", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if created.Email != "User@Example.COM" || created.DisplayName != "User" || created.SessionToken == "" {
		t.Fatalf("unexpected registration: %+v", created)
	}
	if _, err := service.Register(context.Background(), "User@Example.COM", "password123"); !Is(err, ErrEmailTaken) {
		t.Fatalf("expected email conflict, got %v", err)
	}
	if _, err := service.Login(context.Background(), "User@Example.COM", "wrongpass"); !Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	service.random = bytes.NewReader(bytes.Repeat([]byte{3}, 32))
	loggedIn, err := service.Login(context.Background(), "User@Example.COM", "password123")
	if err != nil {
		t.Fatal(err)
	}
	authenticated, err := service.Authenticate(context.Background(), loggedIn.SessionToken)
	if err != nil || authenticated.ID != created.ID {
		t.Fatalf("authenticate login token: %+v %v", authenticated, err)
	}
	updated, err := service.UpdateProfile(context.Background(), authenticated, "New Name", "🙂")
	if err != nil || updated.DisplayName != "New Name" {
		t.Fatalf("update profile: %+v %v", updated, err)
	}
	updated, err = service.DeleteAvatar(context.Background(), updated)
	if err != nil || updated.Avatar != "" {
		t.Fatalf("delete avatar: %+v %v", updated, err)
	}
	if err := service.DeleteUser(context.Background(), updated); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := service.Authenticate(context.Background(), loggedIn.SessionToken); !Is(err, ErrUnauthorized) {
		t.Fatalf("deleted user still authenticates: %v", err)
	}
}

func TestSessionExpiryIdleTimeoutAndLogout(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	service := New(st, &fakeVoice{}, time.Hour, time.Minute, 2, slog.Default()).
		WithSessionPolicy(2*time.Hour, 30*time.Minute)
	service.now = func() time.Time { return now }
	service.random = bytes.NewReader(bytes.Repeat([]byte{4}, 256))
	created, err := service.Register(context.Background(), "session@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), created.SessionToken); err != nil {
		t.Fatalf("fresh session rejected: %v", err)
	}
	now = now.Add(31 * time.Minute)
	if _, err := service.Authenticate(context.Background(), created.SessionToken); !Is(err, ErrUnauthorized) {
		t.Fatalf("idle session accepted: %v", err)
	}

	service.random = bytes.NewReader(bytes.Repeat([]byte{5}, 32))
	loggedIn, err := service.Login(context.Background(), "session@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(context.Background(), loggedIn.SessionToken); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), loggedIn.SessionToken); !Is(err, ErrUnauthorized) {
		t.Fatalf("revoked session accepted: %v", err)
	}

	service.random = bytes.NewReader(bytes.Repeat([]byte{6}, 32))
	loggedIn, err = service.Login(context.Background(), "session@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2*time.Hour + time.Second)
	if _, err := service.Authenticate(context.Background(), loggedIn.SessionToken); !Is(err, ErrUnauthorized) {
		t.Fatalf("expired session accepted: %v", err)
	}
}

func TestSecurityRequiresAnEstablishedSession(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "security.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	service := New(st, &fakeVoice{}, time.Hour, time.Minute, 2, slog.Default())
	service.now = func() time.Time { return now }
	service.random = bytes.NewReader(bytes.Repeat([]byte{7}, 256))
	old, err := service.Register(context.Background(), "security@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(23 * time.Hour)
	if _, err := service.Authenticate(context.Background(), old.SessionToken); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Hour)
	service.random = bytes.NewReader(bytes.Repeat([]byte{8}, 32))
	newSession, err := service.Login(context.Background(), old.Email, "password123")
	if err != nil {
		t.Fatal(err)
	}
	newUser, err := service.Authenticate(context.Background(), newSession.SessionToken)
	if err != nil || newUser.Security == nil || newUser.Security.CanManage {
		t.Fatalf("new session security: %+v %v", newUser.Security, err)
	}
	if err := service.ChangePassword(context.Background(), newUser, newSession.SessionToken, "new-password"); !Is(err, ErrForbidden) {
		t.Fatalf("new session changed password: %v", err)
	}
	oldUser, err := service.Authenticate(context.Background(), old.SessionToken)
	if err != nil || oldUser.Security == nil || !oldUser.Security.Alert {
		t.Fatalf("old session alert: %+v %v", oldUser.Security, err)
	}
	sessions, err := service.SecuritySessions(context.Background(), oldUser, old.SessionToken)
	if err != nil || len(sessions) != 2 {
		t.Fatalf("security sessions: %v %v", sessions, err)
	}
	oldUser, err = service.Authenticate(context.Background(), old.SessionToken)
	if err != nil || oldUser.Security == nil || oldUser.Security.Alert {
		t.Fatalf("review did not clear alert: %+v %v", oldUser.Security, err)
	}
	for _, item := range sessions {
		if !item.Current {
			if err := service.RevokeSecuritySession(context.Background(), oldUser, old.SessionToken, item.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := service.Authenticate(context.Background(), newSession.SessionToken); !Is(err, ErrUnauthorized) {
		t.Fatalf("revoked session authenticated: %v", err)
	}
}

func TestCleanupTerminatesExpiredRooms(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "cleanup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	voice := &fakeVoice{}
	service := New(st, voice, time.Minute, time.Minute, 2, slog.Default())
	now := time.Unix(1_700_000_000, 0).UTC()
	service.now = func() time.Time { return now }
	service.random = bytes.NewReader(bytes.Repeat([]byte{1}, 96))
	owner, err := service.Register(context.Background(), "owner@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	room, err := service.CreateRoom(context.Background(), owner.User)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	service.cleanup(context.Background())
	if len(voice.deleted) != 1 || voice.deleted[0] != room.Code {
		t.Fatalf("cleanup did not terminate room: %v", voice.deleted)
	}
	if _, err := service.GetRoom(context.Background(), room.Code, owner.User.ID); !Is(err, ErrNotFound) {
		t.Fatalf("expired room remains: %v", err)
	}
}

func TestWelcomeStatsUseThePreviousDayOnEveryRequest(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "welcome.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	owner := model.User{ID: "usr_welcome_owner", Email: "owner@example.com", DisplayName: "Owner", CreatedAt: now.Add(-48 * time.Hour)}
	other := model.User{ID: "usr_welcome_other", Email: "other@example.com", DisplayName: "Other", CreatedAt: now.Add(-2 * time.Hour)}
	if err := st.CreateUser(ctx, owner, []byte("owner"), [32]byte{1}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, other, []byte("other"), [32]byte{2}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateGroup(ctx, store.NewGroup{ID: "grp_recent", Visibility: "public", OwnerID: owner.ID, Name: "Recent", Avatar: "👥", InviteToken: "invite_recent"}, now.Add(-2*time.Hour), 0); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateGroup(ctx, store.NewGroup{ID: "grp_old", Visibility: "public", OwnerID: owner.ID, Name: "Old", Avatar: "👥", InviteToken: "invite_old"}, now.Add(-25*time.Hour), 0); err != nil {
		t.Fatal(err)
	}
	if err := st.AddMessage(ctx, model.GroupMessage{ID: "msg_other_recent", GroupID: "grp_recent", Author: other, Body: "recent", CreatedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddMessage(ctx, model.GroupMessage{ID: "msg_owner_recent", GroupID: "grp_recent", Author: owner, Body: "mine", CreatedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddMessage(ctx, model.GroupMessage{ID: "msg_other_old", GroupID: "grp_old", Author: other, Body: "old", CreatedAt: now.Add(-25 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	service := New(st, &fakeVoice{}, time.Hour, time.Minute, 2, slog.Default())
	service.now = func() time.Time { return now }
	stats, err := service.Welcome(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Messages != 2 || stats.NewUsers != 1 || stats.NewGroups != 1 {
		t.Fatalf("unexpected welcome stats: %+v", stats)
	}
	stats, err = service.Welcome(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Messages != 2 || stats.NewUsers != 1 || stats.NewGroups != 1 {
		t.Fatalf("welcome stats changed between requests: %+v", stats)
	}
}
