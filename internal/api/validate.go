package api

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"

	"backend-auth/internal/auth"
)

// Validation limits for account fields.
const (
	MinPasswordLength = 8
	MaxNameLength     = 100
	MaxEmailLength    = 254
)

// normaliseEmail trims and lower-cases an email address.
func normaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// validateSignup checks a signup payload after normalisation.
func validateSignup(name, email, password string) error {
	switch {
	case name == "" || email == "" || password == "":
		return errors.New("name, email and password are required")
	case utf8.RuneCountInString(name) > MaxNameLength:
		return errors.New("name must be at most 100 characters")
	case len(email) > MaxEmailLength || !isEmail(email):
		return errors.New("email address is invalid")
	case utf8.RuneCountInString(password) < MinPasswordLength:
		return errors.New("password must be at least 8 characters")
	case len(password) > auth.MaxPasswordBytes:
		return errors.New("password must be at most 72 bytes")
	}
	return nil
}

// isEmail accepts a bare address such as "a@b.co" (no display name).
func isEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Address == s && strings.Contains(s[strings.LastIndex(s, "@"):], ".")
}
