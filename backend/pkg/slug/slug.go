package slug

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

const (
	slugBytes  = 9  // 9 random bytes → 12 base64url chars (no padding)
	maxRetries = 3
)

// Generate creates a cryptographically random URL-safe slug.
// 9 bytes → base64url → 12 characters [A-Za-z0-9_-]
// Entropy: 72 bits → ~4.7×10²¹ combinations.
func Generate() (string, error) {
	b := make([]byte, slugBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("slug: rand read: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateWithRetry attempts to generate a slug up to maxRetries times.
// The caller should pass a uniqueness-check function that returns true if the slug is already taken.
func GenerateWithRetry(isTaken func(s string) (bool, error)) (string, error) {
	for i := 0; i < maxRetries; i++ {
		s, err := Generate()
		if err != nil {
			return "", err
		}
		taken, err := isTaken(s)
		if err != nil {
			return "", fmt.Errorf("slug: uniqueness check: %w", err)
		}
		if !taken {
			return s, nil
		}
	}
	return "", fmt.Errorf("slug: failed to generate unique slug after %d retries", maxRetries)
}
