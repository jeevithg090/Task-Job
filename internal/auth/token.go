// Package auth provides password hashing and JWT issuing/verification.
package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Roles recognised by the service.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// ErrInvalidToken is returned for any token that fails verification.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims are the JWT claims issued by the service.
type Claims struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// TokenManager issues and verifies HS256-signed JWTs.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewTokenManager returns a TokenManager using the given key and lifetime.
func NewTokenManager(secret []byte, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: secret, ttl: ttl, now: time.Now}
}

// Generate issues a signed token for the given user.
func (m *TokenManager) Generate(userID int64, role string) (string, error) {
	now := m.now()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Validate verifies a token's signature, algorithm and expiry and returns its claims.
func (m *TokenManager) Validate(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil || !token.Valid || claims.UserID <= 0 {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
