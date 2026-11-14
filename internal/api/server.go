// Package api implements the HTTP API: routing, middleware and handlers.
package api

import (
	"context"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"backend-auth/internal/auth"
	"backend-auth/internal/store"
)

// UserStore is the persistence behaviour the handlers depend on.
type UserStore interface {
	CreateUser(ctx context.Context, name, email, passwordHash, role string) (store.User, error)
	UserByEmail(ctx context.Context, email string) (store.User, error)
	UserByID(ctx context.Context, id int64) (store.User, error)
	ListUsers(ctx context.Context) ([]store.User, error)
	Ping(ctx context.Context) error
}

// Server wires handlers to their dependencies.
type Server struct {
	users          UserStore
	tokens         *auth.TokenManager
	allowedOrigins []string
	static         fs.FS
}

// Options configures a Server.
type Options struct {
	Users          UserStore
	Tokens         *auth.TokenManager
	AllowedOrigins []string
	// Static, if non-nil, is served at "/" (the browser API tester).
	Static fs.FS
}

// NewServer returns a Server built from opts.
func NewServer(opts Options) *Server {
	return &Server{
		users:          opts.Users,
		tokens:         opts.Tokens,
		allowedOrigins: opts.AllowedOrigins,
		static:         opts.Static,
	}
}

// Handler returns the fully wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /signup", s.handleSignup)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("GET /profile", s.authenticate(s.handleProfile))
	mux.HandleFunc("GET /users", s.authenticate(requireRole(auth.RoleAdmin, s.handleListUsers)))
	mux.HandleFunc("GET /healthz", s.handleHealth)

	// Without these, "GET /" below would answer GETs to POST-only routes with 404.
	mux.HandleFunc("GET /signup", methodNotAllowed(http.MethodPost))
	mux.HandleFunc("GET /login", methodNotAllowed(http.MethodPost))

	if s.static != nil {
		mux.Handle("GET /", staticHandler(s.static))
	}

	return logRequests(securityHeaders(cors(s.allowedOrigins, mux)))
}

func methodNotAllowed(allow string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", allow)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// staticHandler serves the embedded tester. Paths that don't match a file get
// a 404: the 404.html page for browsers, a JSON error for API clients.
func staticHandler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "" {
			if _, err := fs.Stat(fsys, name); err != nil {
				notFound(w, r, fsys)
				return
			}
		}
		files.ServeHTTP(w, r)
	})
}

func notFound(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		if page, err := fs.ReadFile(fsys, "404.html"); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(page)
			return
		}
	}
	writeError(w, http.StatusNotFound, "not found")
}
