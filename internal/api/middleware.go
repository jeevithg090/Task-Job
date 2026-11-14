package api

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"backend-auth/internal/auth"
)

type contextKey struct{}

// claimsFrom returns the authenticated claims stored by authenticate.
func claimsFrom(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(contextKey{}).(*auth.Claims)
	return c, ok
}

// authenticate rejects requests without a valid "Authorization: Bearer <token>" header.
func (s *Server) authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			writeError(w, http.StatusUnauthorized, "authorization header required")
			return
		}
		scheme, token, ok := strings.Cut(header, " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			writeError(w, http.StatusUnauthorized, "authorization header must use the Bearer scheme")
			return
		}
		claims, err := s.tokens.Validate(strings.TrimSpace(token))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, claims)))
	}
}

// requireRole allows the request only if the authenticated user has role.
// It must be wrapped by authenticate.
func requireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFrom(r.Context())
		if !ok || claims.Role != role {
			writeError(w, http.StatusForbidden, "forbidden: "+role+" access required")
			return
		}
		next(w, r)
	}
}

// cors adds CORS headers for allow-listed origins. With an empty allow-list
// no headers are added and browsers enforce the same-origin policy.
func cors(allowed []string, next http.Handler) http.Handler {
	allowAll := slices.Contains(allowed, "*")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (allowAll || slices.Contains(allowed, origin)) {
			h := w.Header()
			if allowAll {
				h.Set("Access-Control-Allow-Origin", "*")
			} else {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
			}
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			h.Set("Access-Control-Max-Age", "600")
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeaders sets conservative browser security headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"img-src 'self' data:; font-src https://fonts.gstatic.com; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// logRequests logs one line per request and recovers from handler panics.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				slog.Error("panic serving request", "path", r.URL.Path, "panic", p)
				writeError(rec, http.StatusInternalServerError, "internal server error")
			}
			slog.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration", time.Since(start).String(),
			)
		}()
		next.ServeHTTP(rec, r)
	})
}
