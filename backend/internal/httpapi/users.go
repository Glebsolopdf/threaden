package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"voice-rooms/internal/app"
	avatarutil "voice-rooms/internal/avatar"
	"voice-rooms/internal/model"
	"voice-rooms/internal/publicview"
)

type userHandler struct {
	service *app.Service
	groups  interface {
		PublishProfileUpdated(context.Context, model.User)
	}
	cookieSecure bool
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type passwordRequest struct {
	Password string `json:"password"`
}

func (h userHandler) register(w http.ResponseWriter, r *http.Request) {
	request, ok := readAuth(w, r, true)
	if !ok {
		return
	}
	created, err := h.service.Register(r.Context(), request.Email, request.Password)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	h.writeAuth(w, http.StatusCreated, created)
}

func (h userHandler) login(w http.ResponseWriter, r *http.Request) {
	request, ok := readAuth(w, r, false)
	if !ok {
		return
	}
	created, err := h.service.Login(r.Context(), request.Email, request.Password)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	h.writeAuth(w, http.StatusOK, created)
}

func (h userHandler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Logout(r.Context(), sessionToken(r)); err != nil {
		writeAppError(w, r, err)
		return
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func readAuth(w http.ResponseWriter, r *http.Request, registering bool) (authRequest, bool) {
	var request authRequest
	if err := decodeJSON(r, &request); err != nil {
		if isBodyTooLarge(err) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
		} else {
			writeError(w, r, http.StatusBadRequest, "invalid_json", "request body must be one valid JSON object")
		}
		return authRequest{}, false
	}
	request.Email = normalizeEmail(request.Email)
	if !validEmail(request.Email) {
		writeError(w, r, http.StatusBadRequest, "validation_error", "email must be valid")
		return authRequest{}, false
	}
	if !validPassword(request.Password, registering) {
		writeError(w, r, http.StatusBadRequest, "validation_error", "password must contain 6 to 72 characters")
		return authRequest{}, false
	}
	return request, true
}

func (h userHandler) writeAuth(w http.ResponseWriter, status int, created app.CreatedUser) {
	h.setSessionCookie(w, created.SessionToken)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, publicview.OwnUser(created.User))
}

func (h userHandler) setSessionCookie(w http.ResponseWriter, token string) {
	ttl := h.service.SessionTTL()
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/", HttpOnly: true,
		Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode,
		MaxAge: int(ttl.Seconds()), Expires: time.Now().UTC().Add(ttl),
	})
}

func (h userHandler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode,
		MaxAge: -1, Expires: time.Unix(1, 0).UTC(),
	})
}

func (h userHandler) me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, publicview.OwnUser(currentUser(r)))
}

func (h userHandler) welcome(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.Welcome(r.Context(), currentUser(r).ID)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h userHandler) updateProfile(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	name, avatar, ok := readProfile(w, r, user.Avatar)
	if !ok {
		return
	}
	if name == user.DisplayName && avatar == user.Avatar {
		writeError(w, r, http.StatusConflict, "profile_unchanged", "profile has no changes")
		return
	}
	updated, err := h.service.UpdateProfile(r.Context(), user, name, avatar)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	h.groups.PublishProfileUpdated(r.Context(), updated)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, publicview.OwnUser(updated))
}

func (h userHandler) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user.Avatar == "" {
		writeError(w, r, http.StatusConflict, "profile_unchanged", "profile has no changes")
		return
	}
	updated, err := h.service.DeleteAvatar(r.Context(), user)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	h.groups.PublishProfileUpdated(r.Context(), updated)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, publicview.OwnUser(updated))
}

func (h userHandler) deleteUser(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteUser(r.Context(), currentUser(r)); err != nil {
		writeAppError(w, r, err)
		return
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h userHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	var request passwordRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "request body must be one valid JSON object")
		return
	}
	if !validPassword(request.Password, true) {
		writeError(w, r, http.StatusBadRequest, "validation_error", "password must contain 6 to 72 characters")
		return
	}
	if err := h.service.ChangePassword(r.Context(), currentUser(r), sessionToken(r), request.Password); err != nil {
		writeAppError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h userHandler) sessions(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.SecuritySessions(r.Context(), currentUser(r), sessionToken(r))
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h userHandler) revokeSession(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RevokeSecuritySession(r.Context(), currentUser(r), sessionToken(r), chi.URLParam(r, "id")); err != nil {
		writeAppError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readProfile(w http.ResponseWriter, r *http.Request, currentAvatar string) (string, string, bool) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		writeError(w, r, http.StatusBadRequest, "invalid_content_type", "profile update must be multipart/form-data")
		return "", "", false
	}
	parts, err := avatarutil.ReadMultipartProfile(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "upload_error", err.Error())
		return "", "", false
	}
	avatar := currentAvatar
	if parts.HasAvatar {
		avatar = parts.Avatar
	}
	return validProfile(w, r, parts.DisplayName, avatar)
}

func validProfile(w http.ResponseWriter, r *http.Request, name, avatar string) (string, string, bool) {
	name = strings.TrimSpace(name)
	if !validDisplayName(name) {
		writeError(w, r, http.StatusBadRequest, "validation_error", "display_name must contain 1 to 50 characters")
		return "", "", false
	}
	if !avatarutil.ValidStored(avatar) {
		writeError(w, r, http.StatusBadRequest, "validation_error", "avatar is invalid")
		return "", "", false
	}
	return name, avatar, true
}

func decodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errMultipleJSON
		}
		return err
	}
	return nil
}

var errMultipleJSON = &json.SyntaxError{}

func validDisplayName(value string) bool {
	count := utf8.RuneCountInString(value)
	if count < 1 || count > 50 || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func normalizeEmail(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && strings.Contains(value, ".")
}

func validPassword(value string, registering bool) bool {
	return utf8.RuneCountInString(value) >= 6 && utf8.RuneCountInString(value) <= 72 && len(value) <= 72
}
