package groups

import (
	"context"
	"errors"
	"strings"
	"time"

	"voice-rooms/internal/antispam"
	"voice-rooms/internal/groups/hub"
	"voice-rooms/internal/model"
)

func (s *Service) Messages(ctx context.Context, id string, u *model.User, limit int) ([]model.GroupMessage, error) {
	g, e := s.Get(ctx, id, u)
	if e != nil {
		return nil, e
	}
	_ = g
	reader := ""
	if u != nil {
		reader = u.ID
	}
	return s.store.Messages(ctx, id, s.now().Add(-7*24*time.Hour), limit, reader)
}

func (s *Service) MarkRead(ctx context.Context, id, userID, messageID string) error {
	if !s.member(ctx, id, userID) {
		return ErrForbidden
	}
	if messageID == "" {
		return ErrInvalid
	}
	receipts, err := s.store.MarkGroupMessagesRead(ctx, id, userID, messageID, s.now().UTC())
	if err != nil {
		return mapErr(err)
	}
	for _, receipt := range receipts {
		s.hub.Publish([]string{receipt.AuthorID}, hub.Event{Type: "message_read", GroupID: id, Data: hub.MessageReadEvent{MessageID: receipt.MessageID}})
	}
	return nil
}

func (s *Service) Send(ctx context.Context, id string, u model.User, body, idempotencyKey string) (model.GroupMessage, error) {
	if !s.member(ctx, id, u.ID) {
		return model.GroupMessage{}, ErrForbidden
	}
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > 4000 {
		return model.GroupMessage{}, ErrInvalid
	}
	mid, e := s.ID("msg_")
	if e != nil {
		return model.GroupMessage{}, e
	}
	if s.guard != nil {
		var fresh bool
		mid, fresh, e = s.guard.Reserve(ctx, id, u.ID, idempotencyKey, mid)
		if e != nil {
			return model.GroupMessage{}, e
		}
		if !fresh {
			return s.store.Message(ctx, mid)
		}
		if result, e := s.guard.Check(ctx, id, u, body); e != nil {
			if result.DeleteGroup {
				if members, memberErr := s.store.GroupMemberIDs(ctx, id); memberErr == nil {
					_ = s.deleteGroup(ctx, id, members)
				}
			}
			if errors.Is(e, antispam.ErrMessageWarning) {
				return model.GroupMessage{}, ErrWarned
			}
			if errors.Is(e, antispam.ErrMessageRateLimited) {
				return model.GroupMessage{}, ErrLimited
			}
			return model.GroupMessage{}, ErrInvalid
		}
	}
	m := model.GroupMessage{ID: mid, GroupID: id, Author: u, Body: body, CreatedAt: s.now().UTC()}
	if e = s.store.AddMessage(ctx, m); e != nil {
		return m, e
	}
	s.publishGroup(ctx, id, "message_created", m)
	return m, nil
}
