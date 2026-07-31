package model

import "time"

type User struct {
	ID          string         `json:"id"`
	Email       string         `json:"email,omitempty"`
	DisplayName string         `json:"display_name"`
	Avatar      string         `json:"avatar,omitempty"`
	CreatedAt   time.Time      `json:"created_at,omitempty"`
	Security    *SecurityState `json:"security,omitempty"`
}

type SecurityState struct {
	CanManage bool `json:"can_manage"`
	Alert     bool `json:"alert"`
}

type Session struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	Current    bool      `json:"current"`
}

type Member struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Avatar      string    `json:"avatar,omitempty"`
	JoinedAt    time.Time `json:"joined_at"`
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

type Group struct {
	ID             string           `json:"id"`
	Visibility     string           `json:"visibility"`
	Owner          User             `json:"owner"`
	Name           string           `json:"name"`
	Avatar         string           `json:"avatar"`
	InviteToken    string           `json:"invite_token,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	LastActivityAt time.Time        `json:"last_activity_at"`
	MemberCount    int              `json:"member_count"`
	OnlineCount    int              `json:"online_count"`
	LastMessage    *GroupMessage    `json:"last_message,omitempty"`
	VoiceRooms     []GroupVoiceRoom `json:"voice_rooms,omitempty"`
}

type GroupMember struct {
	Member
	Role string `json:"role"`
}

type GroupMessage struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	Author    User      `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type GroupVoiceRoom struct {
	ID               string    `json:"id"`
	GroupID          string    `json:"group_id"`
	Name             string    `json:"name"`
	CreatedAt        time.Time `json:"created_at"`
	ParticipantCount int       `json:"participant_count"`
}
