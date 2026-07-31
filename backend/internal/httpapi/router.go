package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"voice-rooms/internal/abuse"
	"voice-rooms/internal/app"
	appgroups "voice-rooms/internal/groups"
	"voice-rooms/internal/store"
)

type Options struct {
	Origins             []string
	Security            abuse.Config
	SessionCookieSecure bool
}

func New(service *app.Service, groupService *appgroups.Service, st *store.Store, logger *slog.Logger, origins []string) http.Handler {
	return NewWithOptions(service, groupService, st, logger, Options{Origins: origins, Security: abuse.DefaultConfig()})
}

func NewWithSecurity(
	service *app.Service,
	groupService *appgroups.Service,
	st *store.Store,
	logger *slog.Logger,
	origins []string,
	security abuse.Config,
) http.Handler {
	return NewWithOptions(service, groupService, st, logger, Options{Origins: origins, Security: security})
}

func NewWithOptions(
	service *app.Service,
	groupService *appgroups.Service,
	st *store.Store,
	logger *slog.Logger,
	options Options,
) http.Handler {
	security := options.Security
	router := chi.NewRouter()
	limiter := abuse.NewLimiter(st, security)
	router.Use(middleware.RequestID)
	router.Use(requestLogger(logger))
	router.Use(recoverer(logger))
	router.Use(cors(options.Origins))
	router.Use(csrf(options.Origins))
	router.Use(bodyLimit(10 << 20))
	router.Use(rateLimit(limiter, security, logger))
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method is not allowed")
	})

	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Ping(r.Context()); err != nil {
			writeError(w, r, http.StatusServiceUnavailable, "not_ready", "service is not ready")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	users := userHandler{service: service, groups: groupService, cookieSecure: options.SessionCookieSecure}
	rooms := roomHandler{service: service}
	groups := groupHandler{service: groupService}
	router.Post("/v1/auth/register", users.register)
	router.Post("/v1/auth/login", users.login)
	router.Delete("/v1/auth/logout", users.logout)
	router.Get("/v1/discover/groups", groups.discover)
	router.Get("/v1/invites/{token}", groups.invite)
	router.Group(func(public chi.Router) {
		public.Use(optionalAuthenticate(service))
		public.Get("/v1/groups/{id}", groups.get)
		public.Get("/v1/groups/{id}/messages", groups.messages)
	})
	router.Group(func(protected chi.Router) {
		protected.Use(authenticate(service))
		protected.Use(userRateLimit(limiter, security, logger))
		protected.Get("/v1/me", users.me)
		protected.Patch("/v1/me/password", users.changePassword)
		protected.Get("/v1/me/sessions", users.sessions)
		protected.Delete("/v1/me/sessions/{id}", users.revokeSession)
		protected.Patch("/v1/me", users.updateProfile)
		protected.Delete("/v1/me/avatar", users.deleteAvatar)
		protected.Delete("/v1/me", users.deleteUser)
		protected.Post("/v1/rooms", rooms.create)
		protected.Get("/v1/rooms/{code}", rooms.get)
		protected.Post("/v1/rooms/{code}/join", rooms.join)
		protected.Delete("/v1/rooms/{code}/members/me", rooms.leave)
		protected.Delete("/v1/rooms/{code}", rooms.delete)
		protected.Get("/v1/events", groups.events)
		protected.Post("/v1/groups", groups.create)
		protected.Get("/v1/groups", groups.list)
		protected.Get("/v1/groups/{id}/profile", groups.profile)
		protected.Delete("/v1/groups/{id}/members/me", groups.leave)
		protected.Delete("/v1/groups/{id}/members/{memberID}", groups.removeMember)
		protected.Delete("/v1/groups/{id}", groups.delete)
		protected.Post("/v1/groups/{id}/members", groups.join)
		protected.Post("/v1/invites/{token}/join", groups.joinInvite)
		protected.Post("/v1/groups/{id}/messages", groups.send)
		protected.Post("/v1/groups/{id}/voice-rooms", groups.createVoice)
		protected.Post("/v1/group-voice-rooms/{id}/join", groups.joinVoice)
		protected.Delete("/v1/group-voice-rooms/{id}/members/me", groups.leaveVoice)
		protected.Delete("/v1/group-voice-rooms/{id}", groups.deleteVoice)
	})
	return router
}

func routeKey(r *http.Request) string {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) >= 4 && parts[0] == "v1" && parts[1] == "groups" && parts[3] == "messages" {
		return r.Method + " /v1/groups/{id}/messages"
	}
	if len(parts) >= 4 && parts[0] == "v1" && parts[1] == "groups" && parts[3] == "members" {
		return r.Method + " /v1/groups/{id}/members"
	}
	if len(parts) >= 3 && parts[0] == "v1" && parts[1] == "groups" {
		return r.Method + " /v1/groups/{id}"
	}
	if len(parts) >= 4 && parts[0] == "v1" && parts[1] == "rooms" && parts[3] == "join" {
		return r.Method + " /v1/rooms/{code}/join"
	}
	if len(parts) >= 3 && parts[0] == "v1" && parts[1] == "rooms" {
		return r.Method + " /v1/rooms/{code}"
	}
	if len(parts) >= 4 && parts[0] == "v1" && parts[1] == "invites" && parts[3] == "join" {
		return r.Method + " /v1/invites/{token}/join"
	}
	if len(parts) >= 3 && parts[0] == "v1" && parts[1] == "group-voice-rooms" {
		return r.Method + " /v1/group-voice-rooms/{id}/join"
	}
	return r.Method + " " + r.URL.Path
}

type errorBody struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	body.Error.RequestID = middleware.GetReqID(r.Context())
	writeJSON(w, status, body)
}
func writeAppError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case app.Is(err, app.ErrInvalidCredentials):
		writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	case app.Is(err, app.ErrUnauthorized):
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "valid session required")
	case app.Is(err, app.ErrEmailTaken):
		writeError(w, r, http.StatusConflict, "email_taken", "email is already registered")
	case app.Is(err, app.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "operation is not allowed")
	case app.Is(err, app.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
	case app.Is(err, app.ErrRoomFull):
		writeError(w, r, http.StatusConflict, "room_full", "room participant limit reached")
	case app.Is(err, app.ErrRoomCodeUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "room_code_unavailable", "could not allocate room code")
	case app.Is(err, app.ErrLiveKitUnavailable):
		writeError(w, r, http.StatusBadGateway, "livekit_unavailable", "LiveKit is unavailable")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
func isBodyTooLarge(err error) bool { var maxErr *http.MaxBytesError; return errors.As(err, &maxErr) }
