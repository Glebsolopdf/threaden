package groups

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"voice-rooms/internal/model"
	"voice-rooms/internal/store"
)

const maxGroupVoiceRooms = 5

var ErrVoiceLimit = errors.New("group voice room limit reached")

type JoinVoice struct {
	LiveKitURL  string `json:"livekit_url"`
	AccessToken string `json:"access_token"`
	VoiceRoomID string `json:"voice_room_id"`
}

func (s *Service) CreateVoice(ctx context.Context, gid string, u model.User, name string) (model.GroupVoiceRoom, error) {
	g, e := s.store.Group(ctx, gid)
	if e != nil {
		return model.GroupVoiceRoom{}, mapErr(e)
	}
	if g.Owner.ID != u.ID {
		return model.GroupVoiceRoom{}, ErrForbidden
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return model.GroupVoiceRoom{}, ErrInvalid
	}
	id, e := s.ID("gvr_")
	if e != nil {
		return model.GroupVoiceRoom{}, e
	}
	if e = s.store.CreateVoiceRoom(ctx, id, gid, name, s.now().UTC(), maxGroupVoiceRooms); e != nil {
		if store.Is(e, store.ErrRoomFull) {
			return model.GroupVoiceRoom{}, ErrVoiceLimit
		}
		return model.GroupVoiceRoom{}, e
	}
	g, e = s.Get(ctx, gid, &u)
	if e != nil {
		return model.GroupVoiceRoom{}, e
	}
	s.publishGroup(ctx, gid, "group_updated", g)
	return g.VoiceRooms[len(g.VoiceRooms)-1], nil
}

func (s *Service) DeleteVoice(ctx context.Context, id string, u model.User) error {
	r, e := s.store.VoiceRoom(ctx, id)
	if e != nil {
		return mapErr(e)
	}
	g, e := s.store.Group(ctx, r.GroupID)
	if e != nil {
		return mapErr(e)
	}
	if g.Owner.ID != u.ID {
		return ErrForbidden
	}
	if e = s.store.DeleteVoiceRoom(ctx, id); e != nil {
		return mapErr(e)
	}
	if e = s.voice.DeleteRoom(ctx, roomName(r)); e != nil {
		return fmt.Errorf("delete voice room: %w", e)
	}
	updated, e := s.Get(ctx, r.GroupID, &u)
	if e != nil {
		return e
	}
	s.publishGroup(ctx, r.GroupID, "group_updated", updated)
	return nil
}

func (s *Service) JoinVoice(ctx context.Context, id string, u model.User) (JoinVoice, error) {
	r, e := s.store.VoiceRoom(ctx, id)
	if e != nil {
		return JoinVoice{}, mapErr(e)
	}
	if !s.member(ctx, r.GroupID, u.ID) {
		return JoinVoice{}, ErrForbidden
	}
	old, e := s.store.EnterVoice(ctx, id, u.ID, s.now().UTC())
	if e != nil {
		return JoinVoice{}, e
	}
	if old != "" && old != id {
		if oldR, x := s.store.VoiceRoom(ctx, old); x == nil {
			_ = s.voice.RemoveParticipant(ctx, roomName(oldR), u.ID)
		}
	}
	token, e := s.voice.JoinToken(roomName(r), u, 30*time.Minute)
	if e != nil {
		return JoinVoice{}, fmt.Errorf("join token: %w", e)
	}
	s.publishGroup(ctx, r.GroupID, "voice_updated", map[string]string{"voice_room_id": id})
	return JoinVoice{s.voice.PublicURL(), token, id}, nil
}
func (s *Service) LeaveVoice(ctx context.Context, id string, u model.User) error {
	r, e := s.store.VoiceRoom(ctx, id)
	if e != nil {
		return mapErr(e)
	}
	if !s.member(ctx, r.GroupID, u.ID) {
		return ErrForbidden
	}
	if e = s.store.LeaveVoice(ctx, id, u.ID); e != nil {
		return e
	}
	_ = s.voice.RemoveParticipant(ctx, roomName(r), u.ID)
	s.publishGroup(ctx, r.GroupID, "voice_updated", map[string]string{"voice_room_id": id})
	return nil
}
func roomName(r model.GroupVoiceRoom) string { return "group:" + r.GroupID + ":" + r.ID }
