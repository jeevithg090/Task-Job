// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// MinJWTSecretLength is the minimum accepted length, in bytes, of JWT_SECRET.
// HS256 keys shorter than the hash output (32 bytes) weaken the signature.
const MinJWTSecretLength = 32

// Config holds every setting the service needs at runtime.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port string
	// DatabasePath is the SQLite database file location.
	DatabasePath string
	// JWTSecret is the HMAC key used to sign and verify tokens.
	JWTSecret []byte
	// TokenTTL is how long an issued token remains valid.
	TokenTTL time.Duration
	// AllowedOrigins lists origins permitted for cross-origin requests.
	// An empty list disables CORS headers entirely (same-origin only).
	AllowedOrigins []string
	// Admin, when Email is set, describes an administrator account that is
	// created on startup if it does not already exist.
	Admin AdminSeed
}

// AdminSeed describes the bootstrap administrator account.
type AdminSeed struct {
	Name     string
	Email    string
	Password string
}

// Enabled reports whether an admin account should be seeded.
func (a AdminSeed) Enabled() bool { return a.Email != "" }

// Load reads configuration from the process environment.
func Load() (Config, error) {
	return FromLookup(os.LookupEnv)
}

// FromLookup builds a Config using the supplied lookup function, which makes
// configuration parsing testable without touching the real environment.
func FromLookup(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, fallback string) string {
		if v, ok := lookup(key); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		return fallback
	}

	cfg := Config{
		Port:         get("PORT", "8080"),
		DatabasePath: get("DATABASE_PATH", "./users.db"),
		Admin: AdminSeed{
			Name:     get("ADMIN_NAME", "Administrator"),
			Email:    get("ADMIN_EMAIL", ""),
			Password: get("ADMIN_PASSWORD", ""),
		},
	}

	if _, err := strconv.ParseUint(cfg.Port, 10, 16); err != nil {
		return Config{}, fmt.Errorf("PORT must be a valid TCP port, got %q", cfg.Port)
	}

	secret := get("JWT_SECRET", "")
	if secret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}
	if len(secret) < MinJWTSecretLength {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least %d bytes", MinJWTSecretLength)
	}
	cfg.JWTSecret = []byte(secret)

	ttl, err := time.ParseDuration(get("JWT_TTL", "24h"))
	if err != nil || ttl <= 0 {
		return Config{}, errors.New("JWT_TTL must be a positive duration such as 24h or 30m")
	}
	cfg.TokenTTL = ttl

	if origins := get("CORS_ALLOWED_ORIGINS", ""); origins != "" {
		for _, o := range strings.Split(origins, ",") {
			if o = strings.TrimSpace(o); o != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
			}
		}
	}

	if cfg.Admin.Enabled() && cfg.Admin.Password == "" {
		return Config{}, errors.New("ADMIN_PASSWORD is required when ADMIN_EMAIL is set")
	}

	return cfg, nil
}
