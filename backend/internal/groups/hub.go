package groups

import (
	"encoding/json"
	"sync"
)

type Event struct {
	Type    string `json:"type"`
	GroupID string `json:"group_id,omitempty"`
	Data    any    `json:"data,omitempty"`
}
type Hub struct {
	mu    sync.Mutex
	users map[string]map[chan Event]struct{}
}

func NewHub() *Hub { return &Hub{users: map[string]map[chan Event]struct{}{}} }
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
