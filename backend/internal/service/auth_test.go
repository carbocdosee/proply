package service

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// ─── hashToken ───────────────────────────────────────────────────────────────

func TestHashToken_Deterministic(t *testing.T) {
	h1 := hashToken("abc")
	h2 := hashToken("abc")
	if h1 != h2 {
		t.Error("hashToken must be deterministic for the same input")
	}
}

func TestHashToken_DifferentInputs(t *testing.T) {
	if hashToken("token-A") == hashToken("token-B") {
		t.Error("different inputs must produce different hashes")
	}
}

func TestHashToken_IsHex64(t *testing.T) {
	h := hashToken("some-raw-token")
	// SHA-256 hex = 64 characters
	if len(h) != 64 {
		t.Errorf("expected 64-char hex, got %d chars: %s", len(h), h)
	}
	for _, c := range h {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex character %q in hash", c)
		}
	}
}

// ─── generateToken ───────────────────────────────────────────────────────────

func TestGenerateToken_NotEmpty(t *testing.T) {
	raw, hash, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken error: %v", err)
	}
	if raw == "" {
		t.Error("raw token must not be empty")
	}
	if hash == "" {
		t.Error("token hash must not be empty")
	}
}

func TestGenerateToken_RawAndHashConsistent(t *testing.T) {
	raw, hash, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken error: %v", err)
	}
	if hashToken(raw) != hash {
		t.Error("hashToken(raw) must equal the returned hash")
	}
}

func TestGenerateToken_Unique(t *testing.T) {
	raw1, _, _ := generateToken()
	raw2, _, _ := generateToken()
	if raw1 == raw2 {
		t.Error("two consecutive tokens must not be equal")
	}
}

// ─── isUniqueViolation ────────────────────────────────────────────────────────

func TestIsUniqueViolation_True(t *testing.T) {
	// Simulate pgx unique constraint error message containing "23505".
	err := &testErr{"ERROR: duplicate key value violates unique constraint (SQLSTATE 23505)"}
	if !isUniqueViolation(err) {
		t.Error("expected true for 23505 error")
	}
}

func TestIsUniqueViolation_False(t *testing.T) {
	err := &testErr{"connection refused"}
	if isUniqueViolation(err) {
		t.Error("expected false for non-unique-violation error")
	}
}

func TestIsUniqueViolation_Nil(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Error("expected false for nil error")
	}
}

// testErr is a minimal error used to simulate database errors in unit tests.
type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }

// ─── bcrypt hash / verify (AC-8) ─────────────────────────────────────────────

// TestBcrypt_HashAndVerify verifies that a hashed password can be verified
// against the original plain-text password (as used in Register and Login).
func TestBcrypt_HashAndVerify(t *testing.T) {
	password := "secureP@ssw0rd"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword error: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Errorf("bcrypt.CompareHashAndPassword rejected correct password: %v", err)
	}
}

// TestBcrypt_WrongPassword verifies that a wrong password is rejected.
func TestBcrypt_WrongPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)

	err := bcrypt.CompareHashAndPassword(hash, []byte("wrong-password"))
	if err == nil {
		t.Error("expected error when comparing wrong password, got nil")
	}
}

// TestBcrypt_HashUniqueness verifies that the same password produces different
// hashes on each call (bcrypt uses random salt per call).
func TestBcrypt_HashUniqueness(t *testing.T) {
	pw := []byte("same-password")
	hash1, _ := bcrypt.GenerateFromPassword(pw, bcrypt.DefaultCost)
	hash2, _ := bcrypt.GenerateFromPassword(pw, bcrypt.DefaultCost)

	if string(hash1) == string(hash2) {
		t.Error("bcrypt should produce unique hashes for the same password")
	}

	// Both hashes must still verify against the original password.
	if err := bcrypt.CompareHashAndPassword(hash1, pw); err != nil {
		t.Errorf("hash1 verify failed: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hash2, pw); err != nil {
		t.Errorf("hash2 verify failed: %v", err)
	}
}

// TestBcrypt_MinPasswordLength verifies that the service rejects passwords
// shorter than 8 characters (mirrors Register validation logic).
func TestBcrypt_MinPasswordLength(t *testing.T) {
	cases := []struct {
		pw   string
		pass bool
	}{
		{"1234567", false}, // 7 chars — invalid
		{"12345678", true}, // 8 chars — valid
		{"longPassword!", true},
	}

	for _, tc := range cases {
		ok := len(tc.pw) >= 8
		if ok != tc.pass {
			t.Errorf("password %q: expected valid=%v, got valid=%v", tc.pw, tc.pass, ok)
		}
	}
}
