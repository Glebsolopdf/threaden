package groups

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"voice-rooms/internal/antispam"
	"voice-rooms/internal/groups/hub"
	"voice-rooms/internal/model"
	"voice-rooms/internal/publicview"
)

func (s *Service) addMembershipMessage(ctx context.Context, groupID string, actor model.User, action string) (model.GroupMessage, error) {
	id, err := s.ID("msg_")
	if err != nil {
		return model.GroupMessage{}, err
	}
	verb := map[string]string{"joined": "присоединился к чату", "left": "вышел из чата", "removed": "исключён из чата"}[action]
	message := model.GroupMessage{ID: id, GroupID: groupID, Kind: "system", Author: actor, Body: fmt.Sprintf("%s %s", actor.DisplayName, verb), CreatedAt: s.now().UTC()}
	if err := s.store.AddMessage(ctx, message); err != nil {
		return model.GroupMessage{}, err
	}
	s.publishGroup(ctx, groupID, "message_created", publicview.MessageView(message))
	return message, nil
}

func (s *Service) Messages(ctx context.Context, id string, u *model.User, limit int) ([]model.GroupMessage, error) {
	group, err := s.store.Group(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	reader := ""
	cutoff := s.now().Add(-7 * 24 * time.Hour)
	if u != nil {
		reader = u.ID
	}
	if group.Visibility == "private" {
		if u == nil || !s.member(ctx, id, u.ID) {
			return nil, ErrForbidden
		}
		cutoff, err = s.store.GroupMemberJoinedAt(ctx, id, u.ID)
		if err != nil {
			return nil, mapErr(err)
		}
	}
	return s.store.Messages(ctx, id, cutoff, limit, reader)
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
	return s.send(ctx, id, u, body, nil, idempotencyKey)
}

func (s *Service) SendReply(ctx context.Context, id string, u model.User, body, replyToID, idempotencyKey string) (model.GroupMessage, error) {
	var reply *model.MessageReference
	if replyToID != "" {
		original, err := s.store.Message(ctx, replyToID)
		if err != nil || original.GroupID != id {
			return model.GroupMessage{}, ErrInvalid
		}
		reply = &model.MessageReference{ID: original.ID, Author: original.Author, Body: original.Body}
	}
	return s.send(ctx, id, u, body, reply, idempotencyKey)
}

func (s *Service) send(ctx context.Context, id string, u model.User, body string, reply *model.MessageReference, idempotencyKey string) (model.GroupMessage, error) {
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
			if result.IsolateGroup {
				s.isolateMessageAttack(ctx, id, u)
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
	m := model.GroupMessage{ID: mid, GroupID: id, Author: u, Body: body, CreatedAt: s.now().UTC(), ReplyTo: reply}
	if e = s.store.AddMessage(ctx, m); e != nil {
		return m, e
	}
	s.publishGroup(ctx, id, "message_created", publicview.MessageView(m))
	return m, nil
}

func (s *Service) DeleteMessage(ctx context.Context, groupID, id string, u model.User) error {
	message, err := s.store.Message(ctx, id)
	if err != nil {
		return mapErr(err)
	}
	if message.GroupID != groupID {
		return ErrNotFound
	}
	if !s.member(ctx, message.GroupID, u.ID) {
		return ErrForbidden
	}
	group, err := s.store.Group(ctx, message.GroupID)
	if err != nil {
		return mapErr(err)
	}
	if message.Author.ID != u.ID && group.Owner.ID != u.ID {
		return ErrForbidden
	}
	if _, err = s.store.DeleteMessage(ctx, id); err != nil {
		return mapErr(err)
	}
	s.publishGroup(ctx, message.GroupID, "message_deleted", hub.MessageDeletedEvent{MessageID: id})
	return nil
}
