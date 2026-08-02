package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	appgroups "voice-rooms/internal/groups"
	"voice-rooms/internal/groups/hub"
	"voice-rooms/internal/model"
)

type groupHandler struct{ service *appgroups.Service }
type createGroupRequest struct {
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	Visibility string `json:"visibility"`
}
type messageRequest struct {
	Body string `json:"body"`
}
type typingRequest struct {
	Active bool `json:"active"`
}
type voiceRoomRequest struct {
	Name string `json:"name"`
}

func (h groupHandler) create(w http.ResponseWriter, r *http.Request) {
	var v createGroupRequest
	if !decodeGroupJSON(w, r, &v) {
		return
	}
	g, e := h.service.Create(r.Context(), currentUser(r), v.Name, v.Avatar, v.Visibility)
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}
func (h groupHandler) profile(w http.ResponseWriter, r *http.Request) {
	profile, e := h.service.Profile(r.Context(), chi.URLParam(r, "id"), currentUser(r))
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}
func (h groupHandler) delete(w http.ResponseWriter, r *http.Request) {
	if e := h.service.Delete(r.Context(), chi.URLParam(r, "id"), currentUser(r)); e != nil {
		writeGroupError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h groupHandler) leave(w http.ResponseWriter, r *http.Request) {
	if e := h.service.Leave(r.Context(), chi.URLParam(r, "id"), currentUser(r)); e != nil {
		writeGroupError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h groupHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	profile, e := h.service.RemoveMember(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "memberID"), currentUser(r))
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}
func (h groupHandler) list(w http.ResponseWriter, r *http.Request) {
	gs, e := h.service.List(r.Context(), currentUser(r))
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, gs)
}
func (h groupHandler) discover(w http.ResponseWriter, r *http.Request) {
	limit, offset, ok := discoverPage(w, r)
	if !ok {
		return
	}
	gs, e := h.service.Discover(r.Context(), r.URL.Query().Get("q"), limit, offset)
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, gs)
}

func discoverPage(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	limit, offset := 20, 0
	for _, item := range []struct {
		name               string
		target             *int
		minValue, maxValue int
	}{
		{"limit", &limit, 1, 50},
		{"offset", &offset, 0, 1000},
	} {
		value := r.URL.Query().Get(item.name)
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < item.minValue || parsed > item.maxValue {
			writeError(w, r, http.StatusBadRequest, "validation_error", item.name+" is out of range")
			return 0, 0, false
		}
		*item.target = parsed
	}
	return limit, offset, true
}
func (h groupHandler) get(w http.ResponseWriter, r *http.Request) {
	u := optionalUser(r)
	g, e := h.service.Get(r.Context(), chi.URLParam(r, "id"), u)
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, g)
}
func (h groupHandler) messages(w http.ResponseWriter, r *http.Request) {
	u := optionalUser(r)
	limit := 30
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	m, e := h.service.Messages(r.Context(), chi.URLParam(r, "id"), u, limit)
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
func (h groupHandler) typing(w http.ResponseWriter, r *http.Request) {
	var v typingRequest
	if !decodeGroupJSON(w, r, &v) {
		return
	}
	if e := h.service.SetTyping(r.Context(), chi.URLParam(r, "id"), currentUser(r), v.Active); e != nil {
		writeGroupError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h groupHandler) read(w http.ResponseWriter, r *http.Request) { var v struct{ MessageID string `json:"message_id"` }; if !decodeGroupJSON(w, r, &v) { return }; if e := h.service.MarkRead(r.Context(), chi.URLParam(r, "id"), currentUser(r).ID, v.MessageID); e != nil { writeGroupError(w, r, e); return }; w.WriteHeader(http.StatusNoContent) }
func (h groupHandler) join(w http.ResponseWriter, r *http.Request) {
	g, e := h.service.Join(r.Context(), chi.URLParam(r, "id"), currentUser(r), false)
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, g)
}
func (h groupHandler) invite(w http.ResponseWriter, r *http.Request) {
	g, e := h.service.Invite(r.Context(), chi.URLParam(r, "token"))
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, g)
}
func (h groupHandler) joinInvite(w http.ResponseWriter, r *http.Request) {
	preview, e := h.service.Invite(r.Context(), chi.URLParam(r, "token"))
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	g, e := h.service.Join(r.Context(), preview.ID, currentUser(r), true)
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, g)
}
func (h groupHandler) send(w http.ResponseWriter, r *http.Request) {
	var v messageRequest
	if !decodeGroupJSON(w, r, &v) {
		return
	}
	m, e := h.service.Send(r.Context(), chi.URLParam(r, "id"), currentUser(r), v.Body, r.Header.Get("Idempotency-Key"))
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}
func (h groupHandler) createVoice(w http.ResponseWriter, r *http.Request) {
	var v voiceRoomRequest
	if !decodeGroupJSON(w, r, &v) {
		return
	}
	room, e := h.service.CreateVoice(r.Context(), chi.URLParam(r, "id"), currentUser(r), v.Name)
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusCreated, room)
}
func (h groupHandler) joinVoice(w http.ResponseWriter, r *http.Request) {
	join, e := h.service.JoinVoice(r.Context(), chi.URLParam(r, "id"), currentUser(r))
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, join)
}
func (h groupHandler) leaveVoice(w http.ResponseWriter, r *http.Request) {
	e := h.service.LeaveVoice(r.Context(), chi.URLParam(r, "id"), currentUser(r))
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h groupHandler) deleteVoice(w http.ResponseWriter, r *http.Request) {
	e := h.service.DeleteVoice(r.Context(), chi.URLParam(r, "id"), currentUser(r))
	if e != nil {
		writeGroupError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h groupHandler) events(w http.ResponseWriter, r *http.Request) {
	events, stop := h.service.Subscribe(currentUser(r).ID)
	defer stop()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case e := <-events:
			_, _ = w.Write([]byte("event: " + e.Type + "\ndata: " + string(hub.Encode(e)) + "\n\n"))
			flusher.Flush()
		case <-ticker.C:
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
func optionalUser(r *http.Request) *model.User {
	value := r.Context().Value(userContextKey{})
	if value == nil {
		return nil
	}
	u := value.(model.User)
	return &u
}
func decodeGroupJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if e := decodeJSON(r, dst); e != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "request body must be one valid JSON object")
		return false
	}
	return true
}
func writeGroupError(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case appgroups.Is(e, appgroups.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "group or invitation not found")
	case appgroups.Is(e, appgroups.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "group membership is required")
	case appgroups.Is(e, appgroups.ErrInvalid):
		writeError(w, r, http.StatusBadRequest, "validation_error", "group input is invalid")
	case appgroups.Is(e, appgroups.ErrWarned):
		writeError(w, r, http.StatusConflict, "spam_warning", "Не надо, пожалуйста")
	case appgroups.Is(e, appgroups.ErrLimited):
		writeError(w, r, http.StatusTooManyRequests, "rate_limited", "too many requests")
	case appgroups.Is(e, appgroups.ErrGroupLimit):
		writeError(w, r, http.StatusConflict, "group_limit", "user can own at most 3 groups")
	case appgroups.Is(e, appgroups.ErrVoiceLimit):
		writeError(w, r, http.StatusConflict, "voice_room_limit", "group can have at most 5 voice rooms")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
