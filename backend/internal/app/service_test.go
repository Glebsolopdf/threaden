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
	if _, err := service.Login(context.Background(), "User@Example.COM", "wrongpass"); !Is(err, ErrUnauthorized) {
		t.Fatalf("expected login rejection, got %v", err)
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
	service.random = bytes.NewReader(bytes.Repeat([]byte{1}, 64))
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
	if _, err := service.GetRoom(context.Background(), room.Code); !Is(err, ErrNotFound) {
		t.Fatalf("expired room remains: %v", err)
	}
}
