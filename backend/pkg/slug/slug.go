package slug

import (
	"crypto/rand"
	"fmt"
)

const (
	alphabet    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	alphabetLen = byte(len(alphabet)) // 62
	slugLength  = 12
	maxRetries  = 3
	// Rejection threshold: 256 - (256 % 62) = 256 - 8 = 248.
	// Bytes >= 248 are rejected to eliminate modular bias.
	rejectThreshold = 248
)

// Generate creates a cryptographically random Base62 slug.
// 12 characters from [0-9A-Za-z] → ~71 bits of entropy.
func Generate() (string, error) {
	result := make([]byte, 0, slugLength)
	// Read extra bytes upfront to account for rejections (~3.1% per byte).
	// 24 bytes gives negligible probability of not filling 12 slots.
	buf := make([]byte, slugLength*2)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("slug: rand read: %w", err)
	}
	for _, b := range buf {
		if b < rejectThreshold {
			result = append(result, alphabet[b%alphabetLen])
			if len(result) == slugLength {
				return string(result), nil
			}
		}
	}
	// Extremely unlikely fallback: refill if we hit too many rejections.
	for len(result) < slugLength {
		b := make([]byte, 1)
		if _, err := rand.Read(b); err != nil {
			return "", fmt.Errorf("slug: rand read: %w", err)
		}
		if b[0] < rejectThreshold {
			result = append(result, alphabet[b[0]%alphabetLen])
		}
	}
	return string(result), nil
}

// GenerateWithRetry attempts to generate a unique slug up to maxRetries times.
// isTaken must return true if the candidate slug already exists in the store.
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
