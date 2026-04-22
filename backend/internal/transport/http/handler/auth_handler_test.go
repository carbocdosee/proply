package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"proply/internal/config"
	"proply/internal/transport/http/handler"
	pkgjwt "proply/pkg/jwt"
)

// newTestAuthHandler builds an AuthHandler with a nil AuthService and a
// minimal config — sufficient for testing HTTP-layer logic that does not
// reach the service (e.g. JSON parsing errors, missing cookies, OAuth state
// mismatches).  Tests that require the AuthService use the integration build
// tag (see auth_handler_integration_test.go).
func newTestAuthHandler() *handler.AuthHandler {
	jwtMgr := pkgjwt.NewManager("test-secret-aaaabbbbcccc1234", 15)
	cfg := &config.Config{
		AppURL:              "http://localhost:5173",
		Env:                 "test",
		JWTRefreshExpiryDay: 7,
	}
	return handler.NewAuthHandler(nil, jwtMgr, cfg)
}

// ─── Register ────────────────────────────────────────────────────────────────

func TestRegister_InvalidJSON(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader("{bad json"))
	w := httptest.NewRecorder()
	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}

// ─── Login ───────────────────────────────────────────────────────────────────

func TestLogin_InvalidJSON(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	h.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}

// ─── Refresh ─────────────────────────────────────────────────────────────────

func TestRefresh_NoCookie(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	w := httptest.NewRecorder()
	h.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "INVALID_REFRESH_TOKEN")
}

func TestRefresh_InvalidCookieValue(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "not.a.jwt"})
	w := httptest.NewRecorder()
	h.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "INVALID_REFRESH_TOKEN")
}

// ─── MagicLink ───────────────────────────────────────────────────────────────

func TestMagicLink_InvalidJSON(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", strings.NewReader("{"))
	w := httptest.NewRecorder()
	h.MagicLink(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}

// ─── MagicLinkVerify ─────────────────────────────────────────────────────────

// TestMagicLinkVerify_MissingToken verifies that a missing ?token= param
// redirects to the error page (302) — no DB call needed.
func TestMagicLinkVerify_MissingToken(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/magic-link/verify", nil)
	w := httptest.NewRecorder()
	h.MagicLinkVerify(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status: want %d, got %d", http.StatusFound, w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "missing_token") {
		t.Errorf("redirect location should contain 'missing_token', got %q", loc)
	}
}

// ─── VerifyEmail ─────────────────────────────────────────────────────────────

func TestVerifyEmail_MissingToken(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/verify-email", nil)
	w := httptest.NewRecorder()
	h.VerifyEmail(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status: want %d, got %d", http.StatusFound, w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "missing_token") {
		t.Errorf("redirect location should contain 'missing_token', got %q", loc)
	}
}

// ─── GoogleRedirect ──────────────────────────────────────────────────────────

// TestGoogleRedirect_NotConfigured verifies that when Google OAuth credentials
// are not set (googleOAuth == nil), the handler redirects to the login error page.
func TestGoogleRedirect_NotConfigured(t *testing.T) {
	// newTestAuthHandler creates handler with empty GoogleClientID → nil googleOAuth
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google", nil)
	w := httptest.NewRecorder()
	h.GoogleRedirect(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status: want %d, got %d", http.StatusFound, w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "oauth_not_configured") {
		t.Errorf("redirect location should contain 'oauth_not_configured', got %q", loc)
	}
}

// ─── GoogleCallback ──────────────────────────────────────────────────────────

func TestGoogleCallback_NotConfigured(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback", nil)
	w := httptest.NewRecorder()
	h.GoogleCallback(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status: want %d, got %d", http.StatusFound, w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "oauth_not_configured") {
		t.Errorf("redirect location should contain 'oauth_not_configured', got %q", loc)
	}
}

// ─── Logout ──────────────────────────────────────────────────────────────────

func TestLogout_ClearsRefreshCookie(t *testing.T) {
	h := newTestAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	h.Logout(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status: want %d, got %d", http.StatusNoContent, w.Code)
	}

	// Verify the refresh_token cookie is cleared (MaxAge < 0)
	cookies := w.Result().Cookies()
	cleared := false
	for _, c := range cookies {
		if c.Name == "refresh_token" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("expected refresh_token cookie to be cleared (MaxAge < 0)")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// assertErrorCode decodes the JSON body and checks the "code" field.
func assertErrorCode(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]string
	if err := json.NewDecoder(bytes.NewReader(w.Body.Bytes())).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v (body: %s)", err, w.Body.String())
	}
	if body["code"] != want {
		t.Errorf("error code: want %q, got %q", want, body["code"])
	}
}
