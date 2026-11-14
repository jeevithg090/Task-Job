package config

import (
	"strings"
	"testing"
	"time"
)

const validSecret = "0123456789abcdef0123456789abcdef"

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}
}

func TestFromLookupDefaults(t *testing.T) {
	cfg, err := FromLookup(lookupFrom(map[string]string{"JWT_SECRET": validSecret}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" || cfg.DatabasePath != "./users.db" || cfg.TokenTTL != 24*time.Hour {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.Admin.Enabled() || len(cfg.AllowedOrigins) != 0 {
		t.Errorf("admin seed and CORS should be off by default: %+v", cfg)
	}
}

func TestFromLookupOverrides(t *testing.T) {
	cfg, err := FromLookup(lookupFrom(map[string]string{
		"JWT_SECRET":           validSecret,
		"PORT":                 "9090",
		"DATABASE_PATH":        "/data/app.db",
		"JWT_TTL":              "30m",
		"CORS_ALLOWED_ORIGINS": " https://a.example , https://b.example,",
		"ADMIN_EMAIL":          "root@example.com",
		"ADMIN_PASSWORD":       "supersecret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "9090" || cfg.DatabasePath != "/data/app.db" || cfg.TokenTTL != 30*time.Minute {
		t.Errorf("overrides not applied: %+v", cfg)
	}
	if got := strings.Join(cfg.AllowedOrigins, "|"); got != "https://a.example|https://b.example" {
		t.Errorf("origins = %q", got)
	}
	if !cfg.Admin.Enabled() || cfg.Admin.Name != "Administrator" {
		t.Errorf("admin seed = %+v", cfg.Admin)
	}
}

func TestFromLookupErrors(t *testing.T) {
	tests := map[string]map[string]string{
		"missing secret":       {},
		"short secret":         {"JWT_SECRET": "short"},
		"bad port":             {"JWT_SECRET": validSecret, "PORT": "http"},
		"port out of range":    {"JWT_SECRET": validSecret, "PORT": "70000"},
		"bad ttl":              {"JWT_SECRET": validSecret, "JWT_TTL": "forever"},
		"negative ttl":         {"JWT_SECRET": validSecret, "JWT_TTL": "-1h"},
		"admin without passwd": {"JWT_SECRET": validSecret, "ADMIN_EMAIL": "a@b.co"},
	}
	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := FromLookup(lookupFrom(env)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
