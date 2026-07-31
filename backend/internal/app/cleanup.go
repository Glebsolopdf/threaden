package app

import (
	"context"
	"time"
)

type CleanupStats struct {
	InactiveUsersDeleted int
}

func (s *Service) RunCleanup(ctx context.Context, interval time.Duration) {
	s.cleanup(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanup(ctx)
		}
	}
}

func (s *Service) EmergencyCleanup(ctx context.Context, batchSize int) CleanupStats {
	if batchSize < 1 {
		batchSize = 500
	}
	now := s.now()
	s.deleteEmptyRooms(ctx, now, 0)
	s.cleanupExpiredRooms(ctx, now)
	count, err := s.store.DeleteInactiveUsersBatch(ctx, now.Add(-7*24*time.Hour), batchSize)
	if err != nil {
		s.logger.ErrorContext(ctx, "emergency delete inactive users", "error", err)
	}
	return CleanupStats{InactiveUsersDeleted: count}
}

func (s *Service) cleanup(ctx context.Context) {
	now := s.now()
	s.deleteEmptyRooms(ctx, now, 2*time.Minute)
	s.cleanupExpiredRooms(ctx, now)
	if err := s.store.DeleteExpiredSessions(ctx, now); err != nil {
		s.logger.ErrorContext(ctx, "delete expired sessions", "error", err)
	}
	if err := s.store.DeleteInactiveUsers(ctx, now.Add(-7*24*time.Hour)); err != nil {
		s.logger.ErrorContext(ctx, "delete inactive users", "error", err)
	}
}

func (s *Service) cleanupExpiredRooms(ctx context.Context, now time.Time) {
	codes, err := s.store.ExpiredRoomCodes(ctx, now)
	if err != nil {
		s.logger.ErrorContext(ctx, "list expired rooms", "error", err)
		return
	}
	for _, code := range codes {
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := s.voice.DeleteRoom(callCtx, code)
		cancel()
		if err != nil {
			s.logger.WarnContext(ctx, "terminate expired LiveKit room", "room_code", code, "error", err)
			continue
		}
		if err := s.store.DeleteExpiredRoom(ctx, code, now); err != nil {
			s.logger.ErrorContext(ctx, "delete expired room", "room_code", code, "error", err)
		}
	}
}

func (s *Service) deleteEmptyRooms(ctx context.Context, now time.Time, grace time.Duration) {
	codes, err := s.store.EmptyRoomCodes(ctx, now.Add(-grace))
	if err != nil {
		s.logger.ErrorContext(ctx, "list empty rooms", "error", err)
		return
	}
	for _, code := range codes {
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := s.voice.DeleteRoom(callCtx, code)
		cancel()
		if err != nil {
			s.logger.WarnContext(ctx, "terminate empty LiveKit room", "room_code", code, "error", err)
			continue
		}
		if err := s.store.DeleteEmptyRoom(ctx, code, now.Add(-grace)); err != nil {
			s.logger.ErrorContext(ctx, "delete empty room", "room_code", code, "error", err)
		}
	}
}
