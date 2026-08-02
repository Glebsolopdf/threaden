// Package hub implements the presence and realtime event bus for the API.
package hub

import (
	"encoding/json"
	"sync"

	"voice-rooms/internal/model"
)

type Event struct {
	Type    string `json:"type"`
	GroupID string `json:"group_id,omitempty"`
	Data    any    `json:"data,omitempty"`
}

type MemberEvent struct {
	Member EventMember `json:"member"`
}

type TypingEvent struct {
	Member EventMember `json:"member"`
	Active bool        `json:"active"`
}

type MessageReadEvent struct {
	MessageID string `json:"message_id"`
}

type EventMember struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar,omitempty"`
}

func NewMemberEvent(user model.User) MemberEvent {
	return MemberEvent{Member: EventMember{ID: user.ID, DisplayName: user.DisplayName, Avatar: user.Avatar}}
}

type Hub struct {
	mu    sync.Mutex
	users map[string]map[chan Event]struct{}
}

func New() *Hub { return &Hub{users: map[string]map[chan Event]struct{}{}} }
func (h *Hub) Subscribe(userID string) (<-chan Event, func()) {
	ch := make(chan Event, 16)
	h.mu.Lock()
	if h.users[userID] == nil {
		h.users[userID] = map[chan Event]struct{}{}
	}
	h.users[userID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.users[userID], ch)
		if len(h.users[userID]) == 0 {
			delete(h.users, userID)
		}
		close(ch)
		h.mu.Unlock()
	}
}
func (h *Hub) Online(userID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.users[userID]) > 0
}
func (h *Hub) Publish(userIDs []string, event Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range userIDs {
		for ch := range h.users[id] {
			select {
			case ch <- event:
			default:
			}
		}
	}
}
func Encode(e Event) []byte { b, _ := json.Marshal(e); return b }
