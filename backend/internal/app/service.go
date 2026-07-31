package app

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"time"

	"voice-rooms/internal/model"
	"voice-rooms/internal/store"
)

var (
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrNotFound            = errors.New("not found")
	ErrForbidden           = errors.New("forbidden")
	ErrRoomFull            = errors.New("room full")
	ErrRoomCodeUnavailable = errors.New("room code unavailable")
	ErrLiveKitUnavailable  = errors.New("LiveKit unavailable")
)

const (
	defaultSessionTTL     = 7 * 24 * time.Hour
	defaultSessionIdleTTL = 24 * time.Hour
)

type VoiceGateway interface {
	PublicURL() string
	JoinToken(roomCode string, user model.User, ttl time.Duration) (string, error)
	DeleteRoom(ctx context.Context, roomCode string) error
	RemoveParticipant(ctx context.Context, roomCode, identity string) error
}

type Service struct {
	store          *store.Store
	voice          VoiceGateway
	roomTTL        time.Duration
	tokenTTL       time.Duration
	sessionTTL     time.Duration
	sessionIdleTTL time.Duration
	maxMembers     int
	random         io.Reader
	now            func() time.Time
	logger         *slog.Logger
}

func New(
	st *store.Store,
	voice VoiceGateway,
	roomTTL, tokenTTL time.Duration,
	maxMembers int,
	logger *slog.Logger,
) *Service {
	return &Service{
		store: st, voice: voice, roomTTL: roomTTL, tokenTTL: tokenTTL,
		sessionTTL: defaultSessionTTL, sessionIdleTTL: defaultSessionIdleTTL,
		maxMembers: maxMembers, random: rand.Reader, now: time.Now, logger: logger,
	}
}

func (s *Service) WithSessionPolicy(absoluteTTL, idleTTL time.Duration) *Service {
	s.sessionTTL = absoluteTTL
	s.sessionIdleTTL = idleTTL
	return s
}

func (s *Service) SessionTTL() time.Duration { return s.sessionTTL }

func translateStoreError(err error) error {
	switch {
	case store.Is(err, store.ErrNotFound):
		return ErrNotFound
	case store.Is(err, store.ErrForbidden):
		return ErrForbidden
	case store.Is(err, store.ErrRoomFull):
		return ErrRoomFull
	default:
		return err
	}
}
