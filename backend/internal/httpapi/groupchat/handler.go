package groupchat

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"voice-rooms/internal/attachments/storage"
	appgroups "voice-rooms/internal/groups"
	"voice-rooms/internal/model"
	"voice-rooms/internal/publicview"
)

type Hooks struct {
	WriteJSON       func(http.ResponseWriter, int, any)
	WriteError      func(http.ResponseWriter, *http.Request, int, string, string)
	DecodeJSON      func(*http.Request, any) error
	CurrentUser     func(*http.Request) model.User
	OptionalUser    func(*http.Request) *model.User
	WriteGroupError func(http.ResponseWriter, *http.Request, error)
	Attachments     *storage.Service
}

type Handler struct {
	service *appgroups.Service
	hooks   Hooks
}

func New(service *appgroups.Service, hooks Hooks) *Handler {
	return &Handler{service: service, hooks: hooks}
}

type messageRequest struct {
	Body      string `json:"body"`
	ReplyToID string `json:"reply_to_id"`
}
type typingRequest struct {
	Active bool `json:"active"`
}

func (h *Handler) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if e := h.hooks.DecodeJSON(r, dst); e != nil {
		h.hooks.WriteError(w, r, http.StatusBadRequest, "invalid_json", "request body must be one valid JSON object")
		return false
	}
	return true
}

func (h *Handler) Messages(w http.ResponseWriter, r *http.Request) {
	u := h.hooks.OptionalUser(r)
	limit := 30
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			h.hooks.WriteError(w, r, http.StatusBadRequest, "validation_error", "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	m, e := h.service.Messages(r.Context(), chi.URLParam(r, "id"), u, limit)
	if e != nil {
		h.hooks.WriteGroupError(w, r, e)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	h.hooks.WriteJSON(w, http.StatusOK, publicview.Messages(m))
}

func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	if r.MultipartForm != nil || strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		h.sendMultipart(w, r)
		return
	}
	var v messageRequest
	if !h.decode(w, r, &v) {
		return
	}
	m, e := h.service.SendReply(r.Context(), chi.URLParam(r, "id"), h.hooks.CurrentUser(r), v.Body, v.ReplyToID, r.Header.Get("Idempotency-Key"))
	if e != nil {
		h.hooks.WriteGroupError(w, r, e)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	h.hooks.WriteJSON(w, http.StatusCreated, publicview.MessageView(m))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if e := h.service.DeleteMessage(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "messageID"), h.hooks.CurrentUser(r)); e != nil {
		h.hooks.WriteGroupError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Typing(w http.ResponseWriter, r *http.Request) {
	var v typingRequest
	if !h.decode(w, r, &v) {
		return
	}
	if e := h.service.SetTyping(r.Context(), chi.URLParam(r, "id"), h.hooks.CurrentUser(r), v.Active); e != nil {
		h.hooks.WriteGroupError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Read(w http.ResponseWriter, r *http.Request) {
	var v struct {
		MessageID string `json:"message_id"`
	}
	if !h.decode(w, r, &v) {
		return
	}
	if e := h.service.MarkRead(r.Context(), chi.URLParam(r, "id"), h.hooks.CurrentUser(r).ID, v.MessageID); e != nil {
		h.hooks.WriteGroupError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
