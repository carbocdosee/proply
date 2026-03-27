package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT payload for access tokens.
type Claims struct {
	UserID        string `json:"sub"`
	Email         string `json:"email"`
	Plan          string `json:"plan"`
	EmailVerified bool   `json:"email_verified"`
	jwt.RegisteredClaims
}

// Manager handles JWT signing and validation.
type Manager struct {
	secret          []byte
	accessExpiryMin int
}

// NewManager creates a new JWT manager.
func NewManager(secret string, accessExpiryMin int) *Manager {
	return &Manager{
		secret:          []byte(secret),
		accessExpiryMin: accessExpiryMin,
	}
}

// GenerateAccess creates a signed access token for the given user.
func (m *Manager) GenerateAccess(userID, email, plan string, emailVerified bool) (string, error) {
	claims := &Claims{
		UserID:        userID,
		Email:         email,
		Plan:          plan,
		EmailVerified: emailVerified,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(m.accessExpiryMin) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// GenerateRefresh creates a refresh token with a longer expiry.
func (m *Manager) GenerateRefresh(userID string, expiryDays int) (string, error) {
	claims := &jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiryDays) * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ParseAccess parses and validates an access token, returning its claims.
func (m *Manager) ParseAccess(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("jwt: unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwt: parse: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("jwt: invalid token")
	}
	return claims, nil
}

// ParseRefresh parses a refresh token and returns the user ID.
func (m *Manager) ParseRefresh(tokenStr string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("jwt: unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return "", fmt.Errorf("jwt: parse refresh: %w", err)
	}
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("jwt: invalid refresh token")
	}
	return claims.Subject, nil
}
