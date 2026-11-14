package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"backend-auth/internal/auth"
	"backend-auth/internal/store"
)

// EnsureAdmin creates the bootstrap administrator account if no account with
// that email exists yet. An existing account is left untouched.
func EnsureAdmin(ctx context.Context, users UserStore, name, email, password string) error {
	name = strings.TrimSpace(name)
	email = normaliseEmail(email)
	if err := validateSignup(name, email, password); err != nil {
		return fmt.Errorf("invalid admin seed: %w", err)
	}

	existing, err := users.UserByEmail(ctx, email)
	switch {
	case err == nil:
		if existing.Role != auth.RoleAdmin {
			slog.Warn("admin seed email belongs to a non-admin account; leaving it unchanged", "user_id", existing.ID)
		}
		return nil
	case !errors.Is(err, store.ErrNotFound):
		return fmt.Errorf("look up admin: %w", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	user, err := users.CreateUser(ctx, name, email, hash, auth.RoleAdmin)
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	slog.Info("created bootstrap admin account", "user_id", user.ID)
	return nil
}
