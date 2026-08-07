package groups

import (
	"context"
	"time"

	"voice-rooms/internal/groups/hub"
	"voice-rooms/internal/model"
	"voice-rooms/internal/publicview"
)

type IsolationError struct{ Until time.Time }

func (e IsolationError) Error() string { return ErrIsolated.Error() }
func (e IsolationError) Unwrap() error { return ErrIsolated }

func (s *Service) evaluateJoinAttack(ctx context.Context, groupID string) {
	candidates, _, err := s.store.JoinAttackCandidates(ctx, groupID, s.now().UTC())
	if err != nil || len(candidates) == 0 {
		return
	}
	s.applyIsolation(ctx, groupID, candidates)
}

func (s *Service) isolateMessageAttack(ctx context.Context, groupID string, user model.User) {
	if !s.member(ctx, groupID, user.ID) {
		return
	}
	s.applyIsolation(ctx, groupID, []string{user.ID})
}

func (s *Service) applyIsolation(ctx context.Context, groupID string, candidates []string) {
	before, err := s.store.GroupMemberIDs(ctx, groupID)
	if err != nil {
		return
	}
	details, err := s.store.GroupMembers(ctx, groupID, "")
	if err != nil {
		return
	}
	group, removed, err := s.store.IsolateGroup(ctx, groupID, candidates, s.now().UTC())
	if err != nil {
		return
	}
	for _, id := range removed {
		if member := memberEventFrom(details, id); member.ID != "" {
			s.hub.Publish(before, hub.Event{Type: "member_removed", GroupID: groupID, Data: hub.MemberEvent{Member: member}})
		}
	}
	s.hub.Publish(before, hub.Event{Type: "group_updated", GroupID: groupID, Data: publicview.GroupView(group)})
}

func memberEventFrom(members []model.GroupMember, userID string) hub.EventMember {
	for _, member := range members {
		if member.ID == userID {
			return hub.EventMember{ID: member.ID, DisplayName: member.DisplayName, Avatar: member.Avatar}
		}
	}
	return hub.EventMember{}
}
