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

type WelcomeStats struct {
	Messages  int `json:"messages"`
	NewUsers  int `json:"new_users"`
	NewGroups int `json:"new_groups"`
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
	ID                 string           `json:"id"`
	Visibility         string           `json:"visibility"`
	Owner              User             `json:"owner"`
	Name               string           `json:"name"`
	Avatar             string           `json:"avatar"`
	InviteToken        string           `json:"invite_token,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	LastActivityAt     time.Time        `json:"last_activity_at"`
	MemberCount        int              `json:"member_count"`
	OnlineCount        int              `json:"online_count"`
	LastMessage        *GroupMessage    `json:"last_message,omitempty"`
	VoiceRooms         []GroupVoiceRoom `json:"voice_rooms,omitempty"`
	JoinBlocked        bool             `json:"join_blocked"`
	JoinBlockedUntil   *time.Time       `json:"join_blocked_until,omitempty"`
	HistoryVisibleFrom *time.Time       `json:"history_visible_from,omitempty"`
}

type GroupMember struct {
	Member
	Role string `json:"role"`
}

type GroupMessage struct {
	ID          string            `json:"id"`
	GroupID     string            `json:"group_id"`
	Kind        string            `json:"kind,omitempty"`
	Event       string            `json:"event,omitempty"`
	Author      User              `json:"author"`
	Body        string            `json:"body"`
	CreatedAt   time.Time         `json:"created_at"`
	ReplyTo     *MessageReference `json:"reply_to,omitempty"`
	Read        bool              `json:"read,omitempty"`
	Attachments []Attachment      `json:"attachments,omitempty"`
}

type Attachment struct {
	ID        string    `json:"id"`
	MessageID string    `json:"message_id"`
	GroupID   string    `json:"group_id"`
	OwnerID   string    `json:"owner_id"`
	Kind      string    `json:"kind"`
	Mime      string    `json:"mime"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	Path      string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type MessageReference struct {
	ID     string `json:"id"`
	Kind   string `json:"kind,omitempty"`
	Event  string `json:"event,omitempty"`
	Author User   `json:"author"`
	Body   string `json:"body"`
}

type GroupVoiceRoom struct {
	ID               string    `json:"id"`
	GroupID          string    `json:"group_id"`
	Name             string    `json:"name"`
	CreatedAt        time.Time `json:"created_at"`
	ParticipantCount int       `json:"participant_count"`
}

type GroupSpamWarning struct {
	Reason       string    `json:"reason"`
	MessageCount int       `json:"message_count"`
	UserCount    int       `json:"user_count"`
	CreatedAt    time.Time `json:"created_at"`
}
