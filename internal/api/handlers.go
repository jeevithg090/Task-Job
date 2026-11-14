package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"backend-auth/internal/auth"
	"backend-auth/internal/store"
)

type signupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	// Role is accepted only so a clear error can be returned: self-service
	// signup always creates a regular user.
	Role string `json:"role"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type signupResponse struct {
	Message string     `json:"message"`
	User    store.User `json:"user"`
}

type loginResponse struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	email := normaliseEmail(req.Email)
	if err := validateSignup(name, email, req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if role := strings.TrimSpace(req.Role); role != "" && role != auth.RoleUser {
		writeError(w, http.StatusForbidden, "only the user role can be self-registered")
		return
	}

	// Explicit case-insensitive check also covers legacy rows whose unique
	// index predates case-insensitive collation.
	if _, err := s.users.UserByEmail(r.Context(), email); err == nil {
		writeError(w, http.StatusConflict, "email already registered")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		s.internalError(w, "lookup user", err)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.internalError(w, "hash password", err)
		return
	}

	user, err := s.users.CreateUser(r.Context(), name, email, hash, auth.RoleUser)
	if errors.Is(err, store.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		s.internalError(w, "create user", err)
		return
	}

	writeJSON(w, http.StatusCreated, signupResponse{Message: "user created", User: user})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email := normaliseEmail(req.Email)
	if email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := s.users.UserByEmail(r.Context(), email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		auth.EqualiseTiming(req.Password)
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	case err != nil:
		s.internalError(w, "lookup user", err)
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := s.tokens.Generate(user.ID, user.Role)
	if err != nil {
		s.internalError(w, "generate token", err)
		return
	}
	claims, err := s.tokens.Validate(token)
	if err != nil {
		s.internalError(w, "read generated token", err)
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresAt: claims.ExpiresAt.UTC(),
	})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid token claims")
		return
	}
	user, err := s.users.UserByID(r.Context(), claims.UserID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		s.internalError(w, "lookup user", err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.users.ListUsers(r.Context())
	if err != nil {
		s.internalError(w, "list users", err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.users.Ping(ctx); err != nil {
		slog.Error("health check failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) internalError(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}
