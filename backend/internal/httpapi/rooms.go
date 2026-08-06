package httpapi

import (
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"voice-rooms/internal/app"
	"voice-rooms/internal/publicview"
)

var roomCodePattern = regexp.MustCompile(`^[A-HJ-NP-Z2-9]{26}$`)

type roomHandler struct {
	service *app.Service
}

func (h roomHandler) create(w http.ResponseWriter, r *http.Request) {
	room, err := h.service.CreateRoom(r.Context(), currentUser(r))
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, publicview.RoomView(room))
}

func (h roomHandler) get(w http.ResponseWriter, r *http.Request) {
	code, ok := roomCode(w, r)
	if !ok {
		return
	}
	room, err := h.service.GetRoom(r.Context(), code, currentUser(r).ID)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, publicview.RoomView(room))
}

func (h roomHandler) join(w http.ResponseWriter, r *http.Request) {
	code, ok := roomCode(w, r)
	if !ok {
		return
	}
	result, err := h.service.JoinRoom(r.Context(), code, currentUser(r))
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (h roomHandler) leave(w http.ResponseWriter, r *http.Request) {
	code, ok := roomCode(w, r)
	if !ok {
		return
	}
	if err := h.service.LeaveRoom(r.Context(), code, currentUser(r)); err != nil {
		writeAppError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h roomHandler) delete(w http.ResponseWriter, r *http.Request) {
	code, ok := roomCode(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteRoom(r.Context(), code, currentUser(r)); err != nil {
		writeAppError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func roomCode(w http.ResponseWriter, r *http.Request) (string, bool) {
	code := chi.URLParam(r, "code")
	if !roomCodePattern.MatchString(code) {
		writeError(w, r, http.StatusBadRequest, "validation_error", "room code must contain 26 characters from A-H, J-N, P-Z, and 2-9")
		return "", false
	}
	return code, true
}
