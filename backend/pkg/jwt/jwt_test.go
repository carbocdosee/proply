package jwt_test

import (
	"testing"
	"time"

	pkgjwt "proply/pkg/jwt"
)

const testSecret = "test-secret-32-bytes-long-enough!"

func newTestManager() *pkgjwt.Manager {
	return pkgjwt.NewManager(testSecret, 15) // 15-minute access token
}

// ─── GenerateAccess / ParseAccess ────────────────────────────────────────────

func TestGenerateAccess_ValidToken(t *testing.T) {
	m := newTestManager()
	token, err := m.GenerateAccess("user-1", "alice@example.com", "free", false)
	if err != nil {
		t.Fatalf("GenerateAccess error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestParseAccess_RoundTrip(t *testing.T) {
	m := newTestManager()
	token, _ := m.GenerateAccess("user-42", "bob@example.com", "pro", true)

	claims, err := m.ParseAccess(token)
	if err != nil {
		t.Fatalf("ParseAccess error: %v", err)
	}
	if claims.UserID != "user-42" {
		t.Errorf("UserID: want %q, got %q", "user-42", claims.UserID)
	}
	if claims.Email != "bob@example.com" {
		t.Errorf("Email: want %q, got %q", "bob@example.com", claims.Email)
	}
	if claims.Plan != "pro" {
		t.Errorf("Plan: want %q, got %q", "pro", claims.Plan)
	}
	if !claims.EmailVerified {
		t.Error("EmailVerified: want true, got false")
	}
}

func TestParseAccess_WrongSecret(t *testing.T) {
	m1 := pkgjwt.NewManager("secret-one", 15)
	m2 := pkgjwt.NewManager("secret-two", 15)

	token, _ := m1.GenerateAccess("user-1", "a@b.com", "free", false)

	_, err := m2.ParseAccess(token)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestParseAccess_Tampered(t *testing.T) {
	m := newTestManager()
	token, _ := m.GenerateAccess("user-1", "a@b.com", "free", false)
	tampered := token[:len(token)-4] + "XXXX"

	_, err := m.ParseAccess(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token, got nil")
	}
}

func TestParseAccess_Expired(t *testing.T) {
	// Use -1 minute expiry so the token is already expired at issue time.
	m := pkgjwt.NewManager(testSecret, -1)
	token, _ := m.GenerateAccess("user-1", "a@b.com", "free", false)

	_, err := m.ParseAccess(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

// ─── GenerateRefresh / ParseRefresh ─────────────────────────────────────────

func TestGenerateRefresh_ValidToken(t *testing.T) {
	m := newTestManager()
	token, err := m.GenerateRefresh("user-99", 7)
	if err != nil {
		t.Fatalf("GenerateRefresh error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty refresh token")
	}
}

func TestParseRefresh_RoundTrip(t *testing.T) {
	m := newTestManager()
	token, _ := m.GenerateRefresh("user-99", 7)

	userID, err := m.ParseRefresh(token)
	if err != nil {
		t.Fatalf("ParseRefresh error: %v", err)
	}
	if userID != "user-99" {
		t.Errorf("userID: want %q, got %q", "user-99", userID)
	}
}

func TestParseRefresh_WrongSecret(t *testing.T) {
	m1 := pkgjwt.NewManager("secret-A", 15)
	m2 := pkgjwt.NewManager("secret-B", 15)

	token, _ := m1.GenerateRefresh("user-1", 7)

	_, err := m2.ParseRefresh(token)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestParseRefresh_Expired(t *testing.T) {
	// Expired refresh token: 0 days = expires immediately.
	m := pkgjwt.NewManager(testSecret, 15)
	// Build the token manually with a past expiry to simulate expiry.
	_ = time.Second // confirm time package imported
	token, _ := pkgjwt.NewManager(testSecret, 0).GenerateRefresh("user-1", 0)

	_, err := m.ParseRefresh(token)
	if err == nil {
		t.Fatal("expected error for expired refresh token, got nil")
	}
}

func TestParseRefresh_Tampered(t *testing.T) {
	m := newTestManager()
	token, _ := m.GenerateRefresh("user-1", 7)
	tampered := token[:len(token)-3] + "ZZZ"

	_, err := m.ParseRefresh(tampered)
	if err == nil {
		t.Fatal("expected error for tampered refresh token, got nil")
	}
}
