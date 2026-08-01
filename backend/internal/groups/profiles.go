package groups

import (
	"context"
	"time"

	"voice-rooms/internal/groups/hub"
	"voice-rooms/internal/model"
)

type Profile struct {
	Group        model.Group              `json:"group"`
	Members      []model.GroupMember      `json:"members"`
	SpamWarnings []model.GroupSpamWarning `json:"spam_warnings,omitempty"`
}

func (s *Service) Profile(ctx context.Context, groupID string, user model.User) (Profile, error) {
	group, err := s.Get(ctx, groupID, &user)
	if err != nil {
		return Profile{}, err
	}
	members, err := s.store.GroupMembers(ctx, groupID, group.Owner.ID)
	if err != nil {
		return Profile{}, err
	}
	warnings, err := s.store.GroupSpamWarnings(ctx, groupID, s.now().Add(-30*24*time.Hour))
	if err != nil {
		return Profile{}, err
	}
	return Profile{Group: group, Members: members, SpamWarnings: warnings}, nil
}

func (s *Service) Delete(ctx context.Context, groupID string, user model.User) error {
	group, err := s.store.Group(ctx, groupID)
	if err != nil {
		return mapErr(err)
	}
	if group.Owner.ID != user.ID {
		return ErrForbidden
	}
	members, err := s.store.GroupMemberIDs(ctx, groupID)
	if err != nil {
		return err
	}
	return s.deleteGroup(ctx, groupID, members)
}

func (s *Service) Leave(ctx context.Context, groupID string, user model.User) error {
	group, err := s.store.Group(ctx, groupID)
	if err != nil {
		return mapErr(err)
	}
	if group.Owner.ID == user.ID || !s.member(ctx, groupID, user.ID) {
		return ErrForbidden
	}
	members, err := s.store.GroupMemberIDs(ctx, groupID)
	if err != nil {
		return err
	}
	if err := s.store.LeaveGroup(ctx, groupID, user.ID); err != nil {
		return mapErr(err)
	}
	s.hub.Publish(members, hub.Event{Type: "member_left", GroupID: groupID, Data: hub.NewMemberEvent(user)})
	s.publishGroup(ctx, groupID, "group_updated", group)
	return nil
}

func (s *Service) RemoveMember(ctx context.Context, groupID, memberID string, user model.User) (Profile, error) {
	group, err := s.store.Group(ctx, groupID)
	if err != nil {
		return Profile{}, mapErr(err)
	}
	if group.Owner.ID != user.ID || memberID == user.ID || memberID == group.Owner.ID {
		return Profile{}, ErrForbidden
	}
	members, err := s.store.GroupMemberIDs(ctx, groupID)
	if err != nil {
		return Profile{}, err
	}
	groupMembers, err := s.store.GroupMembers(ctx, groupID, group.Owner.ID)
	if err != nil {
		return Profile{}, err
	}
	member := hub.EventMember{ID: memberID}
	for _, item := range groupMembers {
		if item.ID == memberID {
			member = hub.EventMember{ID: item.ID, DisplayName: item.DisplayName, Avatar: item.Avatar}
			break
		}
	}
	if err := s.store.RemoveGroupMember(ctx, groupID, memberID); err != nil {
		return Profile{}, mapErr(err)
	}
	updated, err := s.Profile(ctx, groupID, user)
	if err != nil {
		return Profile{}, err
	}
	s.hub.Publish(members, hub.Event{Type: "member_removed", GroupID: groupID, Data: hub.MemberEvent{Member: member}})
	s.hub.Publish(members, hub.Event{Type: "group_updated", GroupID: groupID, Data: updated.Group})
	return updated, nil
}

func (s *Service) deleteGroup(ctx context.Context, groupID string, members []string) error {
	if err := s.store.DeleteGroup(ctx, groupID); err != nil {
		return mapErr(err)
	}
	s.hub.Publish(members, hub.Event{Type: "group_deleted", GroupID: groupID})
	return nil
}
