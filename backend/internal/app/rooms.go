package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"voice-rooms/internal/model"
	"voice-rooms/internal/store"
)

type JoinResult struct {
	LiveKitURL  string `json:"livekit_url"`
	AccessToken string `json:"access_token"`
	RoomCode    string `json:"room_code"`
}

func (s *Service) CreateRoom(ctx context.Context, owner model.User) (model.Room, error) {
	for range 100 {
		code, err := s.roomCode()
		if err != nil {
			return model.Room{}, err
		}
		err = s.store.InsertRoom(ctx, code, owner.ID, s.now().UTC(), s.roomTTL)
		if err == nil {
			return s.GetRoom(ctx, code)
		}
		if !store.Is(err, store.ErrConflict) {
			return model.Room{}, translateStoreError(err)
		}
	}
	return model.Room{}, ErrRoomCodeUnavailable
}

func (s *Service) roomCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, 26)
	random := make([]byte, len(code))
	if _, err := io.ReadFull(s.random, random); err != nil {
		return "", fmt.Errorf("generate room code: %w", err)
	}
	for i, value := range random {
		code[i] = alphabet[int(value)%len(alphabet)]
	}
	return string(code), nil
}

func (s *Service) GetRoom(ctx context.Context, code string) (model.Room, error) {
	room, err := s.store.GetRoom(ctx, code, s.now(), s.maxMembers)
	return room, translateStoreError(err)
}

func (s *Service) JoinRoom(ctx context.Context, code string, user model.User) (JoinResult, error) {
	now := s.now()
	room, err := s.store.JoinRoom(ctx, code, user.ID, now, s.maxMembers)
	if err != nil {
		return JoinResult{}, translateStoreError(err)
	}
	ttl := minDuration(s.tokenTTL, room.ExpiresAt.Sub(now))
	if ttl <= 0 {
		return JoinResult{}, ErrNotFound
	}
	token, err := s.voice.JoinToken(code, user, ttl)
	if err != nil {
		return JoinResult{}, fmt.Errorf("create join token: %w", err)
	}
	return JoinResult{LiveKitURL: s.voice.PublicURL(), AccessToken: token, RoomCode: code}, nil
}

func (s *Service) LeaveRoom(ctx context.Context, code string, user model.User) error {
	if err := s.store.LeaveRoom(ctx, code, user.ID, s.now()); err != nil {
		return translateStoreError(err)
	}
	if err := s.voice.RemoveParticipant(ctx, code, user.ID); err != nil {
		s.logger.WarnContext(ctx, "could not disconnect participant after leave",
			"room_code", code, "user_id", user.ID, "error", err)
	}
	return nil
}

func (s *Service) DeleteRoom(ctx context.Context, code string, user model.User) error {
	err := s.store.DeleteRoom(ctx, code, user.ID, s.now(), func() error {
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := s.voice.DeleteRoom(callCtx, code); err != nil {
			return fmt.Errorf("%w: %v", ErrLiveKitUnavailable, err)
		}
		return nil
	})
	return translateStoreError(err)
}

func minDuration(values ...time.Duration) time.Duration {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}

func Is(err, target error) bool { return errors.Is(err, target) }
