package groups

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"voice-rooms/internal/abuse"
	avatarutil "voice-rooms/internal/avatar"
	"voice-rooms/internal/model"
	"voice-rooms/internal/store"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrInvalid    = errors.New("invalid group input")
	ErrLimited    = errors.New("group action rate limited")
	ErrVoiceLimit = errors.New("group voice room limit reached")
)

const maxGroupVoiceRooms = 5

type Voice interface {
	PublicURL() string
	JoinToken(string, model.User, time.Duration) (string, error)
	DeleteRoom(context.Context, string) error
	RemoveParticipant(context.Context, string, string) error
}
type Service struct {
	store   *store.Store
	voice   Voice
	hub     *Hub
	now     func() time.Time
	random  io.Reader
	guard   *abuse.MessageGuard
	cleanup CleanupConfig
}

func New(st *store.Store, v Voice, h *Hub) *Service {
	return &Service{
		store: st, voice: v, hub: h, now: time.Now, random: rand.Reader,
		cleanup: DefaultCleanupConfig(),
	}
}
func (s *Service) WithMessageGuard(guard *abuse.MessageGuard) *Service {
	s.guard = guard
	return s
}
func (s *Service) WithCleanup(cfg CleanupConfig) *Service {
	s.cleanup = cfg
	return s
}
func (s *Service) ID(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, e := io.ReadFull(s.random, b); e != nil {
		return "", e
	}
	return prefix + hex.EncodeToString(b), nil
}
func (s *Service) Create(ctx context.Context, u model.User, name, avatar, visibility string) (model.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 || (visibility != "public" && visibility != "private") {
		return model.Group{}, ErrInvalid
	}
	id, e := s.ID("grp_")
	if e != nil {
		return model.Group{}, e
	}
	invite, e := s.ID("inv_")
	if e != nil {
		return model.Group{}, e
	}
	if avatar == "" {
		avatar = "👥"
	} else if !avatarutil.ValidSymbol(avatar) {
		return model.Group{}, ErrInvalid
	}
	now := s.now().UTC()
	e = s.store.CreateGroup(ctx, store.NewGroup{ID: id, Visibility: visibility, OwnerID: u.ID, Name: name, Avatar: avatar, InviteToken: invite}, now)
	if e != nil {
		return model.Group{}, e
	}
	return s.store.Group(ctx, id)
}
func (s *Service) Get(ctx context.Context, id string, u *model.User) (model.Group, error) {
	g, e := s.store.Group(ctx, id)
	if e != nil {
		return g, mapErr(e)
	}
	if g.Visibility == "private" && (u == nil || !s.member(ctx, id, u.ID)) {
		return model.Group{}, ErrForbidden
	}
	return s.withOnline(g), nil
}
func (s *Service) List(ctx context.Context, u model.User) ([]model.Group, error) {
	gs, e := s.store.UserGroups(ctx, u.ID)
	for i := range gs {
		gs[i] = s.withOnline(gs[i])
	}
	return gs, e
}
func (s *Service) Discover(ctx context.Context, q string, limit, offset int) ([]model.Group, error) {
	q = strings.TrimSpace(q)
	if (q != "" && len([]rune(q)) < 2) || len([]rune(q)) > 80 || limit < 1 || limit > 50 || offset < 0 || offset > 1000 {
		return nil, ErrInvalid
	}
	gs, e := s.store.DiscoverGroups(ctx, q, limit, offset)
	for i := range gs {
		gs[i] = s.withOnline(gs[i])
	}
	return gs, e
}
func (s *Service) Invite(ctx context.Context, token string) (model.Group, error) {
	g, e := s.store.GroupByInvite(ctx, token)
	if e != nil {
		return g, mapErr(e)
	}
	g.InviteToken = ""
	return s.withOnline(g), nil
}
func (s *Service) Join(ctx context.Context, id string, u model.User, byInvite bool) (model.Group, error) {
	g, e := s.store.Group(ctx, id)
	if e != nil {
		return model.Group{}, mapErr(e)
	}
	if !byInvite && g.Visibility != "public" {
		return model.Group{}, ErrForbidden
	}
	alreadyMember := s.member(ctx, id, u.ID)
	if e = s.store.JoinGroup(ctx, id, u.ID, s.now().UTC()); e != nil {
		return model.Group{}, mapErr(e)
	}
	if !alreadyMember {
		s.publishGroup(ctx, id, "member_joined", NewMemberEvent(u))
	}
	s.publishGroup(ctx, id, "group_updated", g)
	return s.Get(ctx, id, &u)
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

type JoinVoice struct {
	LiveKitURL  string `json:"livekit_url"`
	AccessToken string `json:"access_token"`
	VoiceRoomID string `json:"voice_room_id"`
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
func (s *Service) member(ctx context.Context, id, user string) bool {
	ok, e := s.store.IsGroupMember(ctx, id, user)
	return e == nil && ok
}
func (s *Service) withOnline(g model.Group) model.Group {
	ids, e := s.store.GroupMemberIDs(context.Background(), g.ID)
	if e != nil {
		return g
	}
	for _, id := range ids {
		if s.hub.Online(id) {
			g.OnlineCount++
		}
	}
	return g
}
func (s *Service) publishGroup(ctx context.Context, id, typ string, data any) {
	ids, e := s.store.GroupMemberIDs(ctx, id)
	if e == nil {
		s.hub.Publish(ids, Event{Type: typ, GroupID: id, Data: data})
	}
}
func (s *Service) PublishProfileUpdated(ctx context.Context, user model.User) {
	groups, err := s.store.UserGroups(ctx, user.ID)
	if err != nil {
		return
	}
	recipients := map[string]struct{}{user.ID: {}}
	for _, group := range groups {
		ids, listErr := s.store.GroupMemberIDs(ctx, group.ID)
		if listErr != nil {
			continue
		}
		for _, id := range ids {
			recipients[id] = struct{}{}
		}
	}
	ids := make([]string, 0, len(recipients))
	for id := range recipients {
		ids = append(ids, id)
	}
	s.hub.Publish(ids, Event{Type: "profile_updated", Data: NewMemberEvent(user)})
}

func (s *Service) Subscribe(userID string) (<-chan Event, func()) {
	wasOnline := s.hub.Online(userID)
	events, stop := s.hub.Subscribe(userID)
	if !wasOnline {
		s.publishPresence(userID)
	}
	return events, func() {
		stop()
		if !s.hub.Online(userID) {
			s.publishPresence(userID)
		}
	}
}
func (s *Service) publishPresence(userID string) {
	items, err := s.store.UserGroups(context.Background(), userID)
	if err != nil {
		return
	}
	for _, group := range items {
		s.publishGroup(context.Background(), group.ID, "presence_updated", s.withOnline(group))
	}
}
func (s *Service) Cleanup(ctx context.Context) {
	_ = s.store.DeleteExpiredMessages(ctx, s.now().Add(-7*24*time.Hour))
	s.cleanupInactiveGroups(ctx)
}
func mapErr(e error) error {
	if store.Is(e, store.ErrNotFound) {
		return ErrNotFound
	}
	return e
}
func Is(err, target error) bool { return errors.Is(err, target) }
