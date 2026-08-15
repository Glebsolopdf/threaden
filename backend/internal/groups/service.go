package groups

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"time"

	"voice-rooms/internal/antispam"
	avatarutil "voice-rooms/internal/avatar"
	groupcleanup "voice-rooms/internal/groups/cleanup"
	groupevents "voice-rooms/internal/groups/events"
	"voice-rooms/internal/groups/hub"
	"voice-rooms/internal/model"
	"voice-rooms/internal/publicview"
	"voice-rooms/internal/store"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrInvalid    = errors.New("invalid group input")
	ErrLimited    = errors.New("group action rate limited")
	ErrWarned     = errors.New("group action warned")
	ErrGroupLimit = errors.New("group limit reached")
	ErrIsolated   = errors.New("group is temporarily isolated")
)

type Voice interface {
	PublicURL() string
	JoinToken(string, model.User, time.Duration) (string, error)
	DeleteRoom(context.Context, string) error
	RemoveParticipant(context.Context, string, string) error
}

type Limits struct {
	MaxUserGroups      int
	DiscoverMinMembers int
}

type CleanupConfig = groupcleanup.Config
type EmergencyCleanupConfig = groupcleanup.EmergencyConfig
type EmergencyCleanupStats = groupcleanup.Stats

func DefaultCleanupConfig() CleanupConfig { return groupcleanup.DefaultConfig() }

func DefaultLimits() Limits { return Limits{MaxUserGroups: 3, DiscoverMinMembers: 5} }

type Service struct {
	store   *store.Store
	voice   Voice
	hub     *hub.Hub
	now     func() time.Time
	random  io.Reader
	guard   *antispam.Guard
	cleanup CleanupConfig
	limits  Limits
}

func New(st *store.Store, v Voice, h *hub.Hub) *Service {
	return &Service{
		store: st, voice: v, hub: h, now: time.Now, random: rand.Reader,
		cleanup: DefaultCleanupConfig(),
		limits:  DefaultLimits(),
	}
}
func (s *Service) WithMessageGuard(guard *antispam.Guard) *Service {
	s.guard = guard
	return s
}
func (s *Service) WithCleanup(cfg CleanupConfig) *Service {
	s.cleanup = cfg
	return s
}
func (s *Service) WithLimits(limits Limits) *Service {
	s.limits = limits
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
	e = s.store.CreateGroup(ctx, store.NewGroup{ID: id, Visibility: visibility, OwnerID: u.ID, Name: name, Avatar: avatar, InviteToken: invite}, now, s.limits.MaxUserGroups)
	if e != nil {
		if store.Is(e, store.ErrGroupLimit) {
			return model.Group{}, ErrGroupLimit
		}
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
		return model.Group{}, ErrNotFound
	}
	g = s.withOnline(g)
	if g.Visibility == "private" && u != nil && u.ID != g.Owner.ID && s.member(ctx, id, u.ID) {
		if joined, joinErr := s.store.GroupMemberJoinedAt(ctx, id, u.ID); joinErr == nil && joined.After(g.CreatedAt) {
			g.HistoryVisibleFrom = &joined
		}
	}
	return g, nil
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
	gs, e := s.store.DiscoverGroups(ctx, q, s.limits.DiscoverMinMembers, limit, offset)
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
func (s *Service) Join(ctx context.Context, id string, u model.User, byInvite bool, ip string) (model.Group, error) {
	g, e := s.store.Group(ctx, id)
	if e != nil {
		return model.Group{}, mapErr(e)
	}
	now := s.now().UTC()
	alreadyMember := s.member(ctx, id, u.ID)
	_ = s.store.RecordJoinEvent(ctx, id, u.ID, ip, false, now)
	if !byInvite && g.Visibility != "public" {
		return model.Group{}, ErrForbidden
	}
	if g.JoinBlocked && !alreadyMember {
		return model.Group{}, IsolationError{Until: *g.JoinBlockedUntil}
	}
	if e = s.store.JoinGroup(ctx, id, u.ID, now); e != nil {
		return model.Group{}, mapErr(e)
	}
	if !alreadyMember {
		_ = s.store.RecordJoinEvent(ctx, id, u.ID, ip, true, now)
		s.evaluateJoinAttack(ctx, id)
	}
	if !alreadyMember {
		if _, e = s.addMembershipMessage(ctx, id, u, "joined"); e != nil {
			return model.Group{}, e
		}
		s.publishGroup(ctx, id, "member_joined", hub.NewMemberEvent(u))
	}
	s.publishGroup(ctx, id, "group_updated", publicview.GroupView(g))
	return s.Get(ctx, id, &u)
}
func (s *Service) member(ctx context.Context, id, user string) bool {
	ok, e := s.store.IsGroupMember(ctx, id, user)
	return e == nil && ok
}

func (s *Service) SetTyping(ctx context.Context, id string, u model.User, active bool) error {
	if !s.member(ctx, id, u.ID) {
		return ErrForbidden
	}
	ids, err := s.store.GroupMemberIDs(ctx, id)
	if err != nil {
		return err
	}
	s.hub.Publish(ids, hub.Event{
		Type: "typing_updated", GroupID: id,
		Data: hub.TypingEvent{Member: hub.NewMemberEvent(u).Member, Active: active},
	})
	return nil
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
		s.hub.Publish(ids, hub.Event{Type: typ, GroupID: id, Data: publicEventData(data)})
	}
}

func publicEventData(data any) any {
	switch value := data.(type) {
	case model.Group:
		return publicview.GroupView(value)
	case model.GroupMessage:
		return publicview.MessageView(value)
	default:
		return data
	}
}
func (s *Service) PublishMessageDeleted(ctx context.Context, groupID, messageID string) error {
	return groupevents.PublishMessageDeleted(ctx, s.store, s.hub, groupID, messageID)
}

func (s *Service) Subscribe(userID string) (<-chan hub.Event, func()) {
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
		s.publishGroup(context.Background(), group.ID, "presence_updated", publicview.GroupView(s.withOnline(group)))
	}
}
func (s *Service) Cleanup(ctx context.Context) {
	_ = s.store.DeleteExpiredMessages(ctx, s.now().Add(-7*24*time.Hour))
	s.cleanupInactiveGroups(ctx)
}

func (s *Service) cleanupInactiveGroups(ctx context.Context) {
	groupcleanup.Run(ctx, s.store, s.now(), s.cleanup)
}

func (s *Service) EmergencyCleanup(ctx context.Context, cfg EmergencyCleanupConfig) EmergencyCleanupStats {
	if cfg.Logger == nil {
		cfg.Logger = s.cleanup.Logger
	}
	return groupcleanup.Emergency(ctx, s.store, s.now(), cfg)
}
func mapErr(e error) error {
	if store.Is(e, store.ErrNotFound) {
		return ErrNotFound
	}
	return e
}
func Is(err, target error) bool { return errors.Is(err, target) }
