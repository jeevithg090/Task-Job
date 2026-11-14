package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "correct horse" {
		t.Fatal("hash must not equal the plaintext")
	}
	if !CheckPassword(hash, "correct horse") {
		t.Error("expected matching password to verify")
	}
	if CheckPassword(hash, "wrong horse") {
		t.Error("expected wrong password to fail")
	}
}

func TestHashPasswordTooLong(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("a", MaxPasswordBytes+1)); err != ErrPasswordTooLong {
		t.Fatalf("err = %v, want ErrPasswordTooLong", err)
	}
}

func TestTokenRoundTrip(t *testing.T) {
	m := NewTokenManager(secret, time.Hour)
	tok, err := m.Generate(42, RoleAdmin)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := m.Validate(tok)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.UserID != 42 || claims.Role != RoleAdmin || claims.Subject != "42" {
		t.Errorf("claims = %+v", claims)
	}
}

func TestTokenRejected(t *testing.T) {
	m := NewTokenManager(secret, time.Hour)
	good, _ := m.Generate(1, RoleUser)

	expired := NewTokenManager(secret, time.Hour)
	expired.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	expiredTok, _ := expired.Generate(1, RoleUser)

	otherKey, _ := NewTokenManager([]byte("ffffffffffffffffffffffffffffffff"), time.Hour).Generate(1, RoleUser)

	noneTok, _ := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{
		UserID:           1,
		Role:             RoleAdmin,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, Claims{
		UserID:           1,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString(secret)

	noExp, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{UserID: 1}).SignedString(secret)

	cases := map[string]string{
		"garbage":       "not-a-token",
		"tampered":      good[:len(good)-2] + "xx",
		"expired":       expiredTok,
		"wrong key":     otherKey,
		"alg none":      noneTok,
		"alg HS512":     hs512,
		"no expiration": noExp,
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := m.Validate(tok); err != ErrInvalidToken {
				t.Fatalf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}
