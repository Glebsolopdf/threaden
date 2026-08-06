package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"voice-rooms/internal/abuse"
	"voice-rooms/internal/app"
	"voice-rooms/internal/clientip"
	"voice-rooms/internal/model"
)

type userContextKey struct{}

const sessionCookieName = "threaden_session"

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)
			logger.InfoContext(r.Context(), "http request",
				"method", r.Method, "path", r.URL.Path, "status", sw.status,
				"bytes", sw.bytes, "duration", time.Since(start), "client_ip", maskedIP(clientIP(r, nil)))
		})
	}
}

func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if value := recover(); value != nil {
					logger.ErrorContext(r.Context(), "panic recovered", "panic", value)
					writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func bodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				writeError(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func rateLimit(limiter *abuse.Limiter, cfg abuse.Config, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := routeKey(r)
			limit := abuse.EndpointLimit(cfg, route)
			checks := []struct {
				scope, subject string
				limit          abuse.Limit
			}{
				{"global", "service", cfg.GlobalLimit},
				{"ip:" + route, abuse.Subject("ip", clientIP(r, cfg.TrustedProxies)), limit},
			}
			if token := sessionToken(r); token != "" {
				checks = append(checks, struct {
					scope, subject string
					limit          abuse.Limit
				}{"session:" + route, sessionSubject(token), limit})
			}
			for _, check := range checks {
				if !allowRequest(w, r, limiter, logger, check.scope, check.subject, check.limit) {
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func userRateLimit(limiter *abuse.Limiter, cfg abuse.Config, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := routeKey(r)
			if !allowRequest(w, r, limiter, logger, "user:"+route, abuse.Subject("user", currentUser(r).ID), abuse.EndpointLimit(cfg, route)) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func allowRequest(w http.ResponseWriter, r *http.Request, limiter *abuse.Limiter, logger *slog.Logger, scope, subject string, limit abuse.Limit) bool {
	decision, err := limiter.Allow(r.Context(), scope, subject, limit)
	if err != nil {
		logger.ErrorContext(r.Context(), "rate limit failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
		return false
	}
	if decision.Allowed {
		return true
	}
	seconds := max(1, int(decision.RetryAfter.Seconds()))
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	logger.WarnContext(r.Context(), "rate limit exceeded", "scope", scope, "key", decision.Key)
	writeError(w, r, http.StatusTooManyRequests, "rate_limited", "too many requests")
	return false
}

func cors(origins []string) func(http.Handler) http.Handler {
	allowAll := slices.Contains(origins, "*")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (allowAll || slices.Contains(origins, origin)) {
				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Add("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func csrf(origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions ||
				bearerToken(r) != "" || !hasSessionCookie(r) {
				next.ServeHTTP(w, r)
				return
			}
			origin := r.Header.Get("Origin")
			if origin == "" && r.Header.Get("Sec-Fetch-Site") != "cross-site" {
				next.ServeHTTP(w, r)
				return
			}
			scheme := "http"
			if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				scheme = "https"
			}
			if origin != scheme+"://"+r.Host && !slices.Contains(origins, origin) {
				writeError(w, r, http.StatusForbidden, "csrf_rejected", "request origin is not allowed")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hasSessionCookie(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	return err == nil && cookie.Value != ""
}

func bearerToken(r *http.Request) string {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}

func authenticate(service *app.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := sessionToken(r)
			if token == "" {
				writeAppError(w, r, app.ErrUnauthorized)
				return
			}
			user, err := service.Authenticate(r.Context(), token)
			if err != nil {
				writeAppError(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, user)))
		})
	}
}

func optionalAuthenticate(service *app.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := sessionToken(r)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			user, err := service.Authenticate(r.Context(), token)
			if err != nil {
				writeAppError(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, user)))
		})
	}
}

func currentUser(r *http.Request) model.User {
	return r.Context().Value(userContextKey{}).(model.User)
}

func sessionToken(r *http.Request) string {
	if token := bearerToken(r); token != "" {
		return token
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}

func sessionSubject(token string) string {
	hash := sha256.Sum256([]byte(token))
	return abuse.Subject("session", hex.EncodeToString(hash[:8]))
}

func clientIP(r *http.Request, trusted []string) string {
	return clientip.Resolve(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), trusted)
}

func maskedIP(value string) string {
	ip := net.ParseIP(value)
	if ip == nil {
		return "unknown"
	}
	if v4 := ip.To4(); v4 != nil {
		return net.IPv4(v4[0], v4[1], v4[2], 0).String()
	}
	return ip.Mask(net.CIDRMask(64, 128)).String()
}
