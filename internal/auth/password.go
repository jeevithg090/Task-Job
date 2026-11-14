package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// MaxPasswordBytes is bcrypt's input limit; longer passwords are rejected
// rather than silently truncated.
const MaxPasswordBytes = 72

// ErrPasswordTooLong is returned when a password exceeds MaxPasswordBytes.
var ErrPasswordTooLong = errors.New("password exceeds 72 bytes")

// dummyHash is compared against when a login targets an unknown account so
// that response timing does not reveal whether the email is registered.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equaliser"), bcrypt.DefaultCost)

// HashPassword returns a bcrypt hash of password.
func HashPassword(password string) (string, error) {
	if len(password) > MaxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether password matches the bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// EqualiseTiming performs a throwaway bcrypt comparison. Call it on code
// paths that reject a login before a real comparison would have happened.
func EqualiseTiming(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}
