// Package publicview contains the explicit JSON shapes exposed to other users.
package publicview

import (
	"time"

	"voice-rooms/internal/model"
)

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar,omitempty"`
}
type SelfUser struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Avatar      string    `json:"avatar,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}
type Member struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Avatar      string    `json:"avatar,omitempty"`
	JoinedAt    time.Time `json:"joined_at"`
}
type MessageReference struct {
	ID     string `json:"id"`
	Author User   `json:"author"`
	Body   string `json:"body"`
}
type Message struct {
	ID        string            `json:"id"`
	GroupID   string            `json:"group_id"`
	Author    User              `json:"author"`
	Body      string            `json:"body"`
	CreatedAt time.Time         `json:"created_at"`
	ReplyTo   *MessageReference `json:"reply_to,omitempty"`
	Read      bool              `json:"read,omitempty"`
}
type VoiceRoom struct {
	ID               string    `json:"id"`
	GroupID          string    `json:"group_id"`
	Name             string    `json:"name"`
	CreatedAt        time.Time `json:"created_at"`
	ParticipantCount int       `json:"participant_count"`
}
type Group struct {
	ID             string      `json:"id"`
	Visibility     string      `json:"visibility"`
	Owner          User        `json:"owner"`
	Name           string      `json:"name"`
	Avatar         string      `json:"avatar"`
	InviteToken    string      `json:"invite_token,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	LastActivityAt time.Time   `json:"last_activity_at"`
	MemberCount    int         `json:"member_count"`
	OnlineCount    int         `json:"online_count"`
	LastMessage    *Message    `json:"last_message,omitempty"`
	VoiceRooms     []VoiceRoom `json:"voice_rooms,omitempty"`
}
type GroupMember struct {
	Member
	Role string `json:"role"`
}
type Profile struct {
	Group        Group                    `json:"group"`
	Members      []GroupMember            `json:"members"`
	SpamWarnings []model.GroupSpamWarning `json:"spam_warnings,omitempty"`
}
type Room struct {
	Code             string    `json:"code"`
	Owner            User      `json:"owner"`
	CreatedAt        time.Time `json:"created_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	ParticipantCount int       `json:"participant_count"`
	MaxParticipants  int       `json:"max_participants"`
	Members          []Member  `json:"members"`
}

func PublicUser(u model.User) User {
	return User{ID: u.ID, DisplayName: u.DisplayName, Avatar: u.Avatar}
}
func OwnUser(u model.User) SelfUser {
	return SelfUser{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Avatar: u.Avatar, CreatedAt: u.CreatedAt}
}
func MessageView(m model.GroupMessage) Message {
	var reply *MessageReference
	if m.ReplyTo != nil {
		reply = &MessageReference{ID: m.ReplyTo.ID, Author: PublicUser(m.ReplyTo.Author), Body: m.ReplyTo.Body}
	}
	return Message{ID: m.ID, GroupID: m.GroupID, Author: PublicUser(m.Author), Body: m.Body, CreatedAt: m.CreatedAt, ReplyTo: reply, Read: m.Read}
}
func Messages(items []model.GroupMessage) []Message {
	out := make([]Message, len(items))
	for i := range items {
		out[i] = MessageView(items[i])
	}
	return out
}
func GroupView(g model.Group) Group {
	var last *Message
	if g.LastMessage != nil {
		value := MessageView(*g.LastMessage)
		last = &value
	}
	rooms := make([]VoiceRoom, len(g.VoiceRooms))
	for i, r := range g.VoiceRooms {
		rooms[i] = VoiceRoom{ID: r.ID, GroupID: r.GroupID, Name: r.Name, CreatedAt: r.CreatedAt, ParticipantCount: r.ParticipantCount}
	}
	return Group{ID: g.ID, Visibility: g.Visibility, Owner: PublicUser(g.Owner), Name: g.Name, Avatar: g.Avatar, CreatedAt: g.CreatedAt, LastActivityAt: g.LastActivityAt, MemberCount: g.MemberCount, OnlineCount: g.OnlineCount, LastMessage: last, VoiceRooms: rooms}
}
func GroupWithInvite(g model.Group) Group {
	view := GroupView(g)
	view.InviteToken = g.InviteToken
	return view
}
func Groups(items []model.Group) []Group {
	out := make([]Group, len(items))
	for i := range items {
		out[i] = GroupView(items[i])
	}
	return out
}
func ProfileView(g model.Group, members []model.GroupMember, warnings []model.GroupSpamWarning) Profile {
	out := make([]GroupMember, len(members))
	for i, m := range members {
		out[i] = GroupMember{Member: Member{ID: m.ID, DisplayName: m.DisplayName, Avatar: m.Avatar, JoinedAt: m.JoinedAt}, Role: m.Role}
	}
	return Profile{Group: GroupWithInvite(g), Members: out, SpamWarnings: warnings}
}
func RoomView(r model.Room) Room {
	members := make([]Member, len(r.Members))
	for i, m := range r.Members {
		members[i] = Member{ID: m.ID, DisplayName: m.DisplayName, Avatar: m.Avatar, JoinedAt: m.JoinedAt}
	}
	return Room{Code: r.Code, Owner: PublicUser(r.Owner), CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, ParticipantCount: r.ParticipantCount, MaxParticipants: r.MaxParticipants, Members: members}
}
