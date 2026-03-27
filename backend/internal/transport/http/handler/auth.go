package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"proply/internal/config"
	"proply/internal/service"
	pkgjwt "proply/pkg/jwt"
)

const oauthStateCookieName = "oauth_state"

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authSvc     *service.AuthService
	jwtManager  *pkgjwt.Manager
	cfg         *config.Config
	googleOAuth *oauth2.Config
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authSvc *service.AuthService, jwtManager *pkgjwt.Manager, cfg *config.Config) *AuthHandler {
	var googleOAuth *oauth2.Config
	if cfg.GoogleClientID != "" {
		googleOAuth = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		}
	}
	return &AuthHandler{
		authSvc:     authSvc,
		jwtManager:  jwtManager,
		cfg:         cfg,
		googleOAuth: googleOAuth,
	}
}

// Register handles POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	user, err := h.authSvc.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		switch err {
		case service.ErrEmailExists:
			respondError(w, http.StatusConflict, "EMAIL_EXISTS")
		case service.ErrValidation:
			respondError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	accessToken, err := h.jwtManager.GenerateAccess(user.ID, user.Email, string(user.Plan), user.EmailVerifiedAt != nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "TOKEN_ERROR")
		return
	}
	refreshToken, err := h.jwtManager.GenerateRefresh(user.ID, h.cfg.JWTRefreshExpiryDay)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "TOKEN_ERROR")
		return
	}
	h.setRefreshCookie(w, refreshToken)
	respond(w, http.StatusCreated, map[string]any{
		"access_token":   accessToken,
		"email_verified": user.EmailVerifiedAt != nil,
	})
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	user, err := h.authSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS")
		return
	}

	accessToken, err := h.jwtManager.GenerateAccess(user.ID, user.Email, string(user.Plan), user.EmailVerifiedAt != nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "TOKEN_ERROR")
		return
	}

	refreshToken, err := h.jwtManager.GenerateRefresh(user.ID, h.cfg.JWTRefreshExpiryDay)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "TOKEN_ERROR")
		return
	}

	h.setRefreshCookie(w, refreshToken)
	respondOK(w, map[string]any{
		"access_token":   accessToken,
		"email_verified": user.EmailVerifiedAt != nil,
	})
}

// Refresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		respondError(w, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN")
		return
	}

	userID, err := h.jwtManager.ParseRefresh(cookie.Value)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN")
		return
	}

	user, err := h.authSvc.GetByID(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN")
		return
	}

	accessToken, err := h.jwtManager.GenerateAccess(user.ID, user.Email, string(user.Plan), user.EmailVerifiedAt != nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "TOKEN_ERROR")
		return
	}

	respondOK(w, map[string]any{
		"access_token":   accessToken,
		"email_verified": user.EmailVerifiedAt != nil,
	})
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /api/v1/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	user, err := h.authSvc.GetByID(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}

	respondOK(w, user)
}

// ResendVerification handles POST /api/v1/auth/resend-verification
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	if claims.EmailVerified {
		respondError(w, http.StatusConflict, "ALREADY_VERIFIED")
		return
	}

	user, err := h.authSvc.GetByID(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}

	if err := h.authSvc.SendVerificationEmail(r.Context(), user.ID, user.Email); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	respondOK(w, map[string]string{"message": "verification email sent"})
}

// VerifyEmail handles GET /api/v1/auth/verify-email?token=...
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/verify-email?error=missing_token", http.StatusFound)
		return
	}

	user, err := h.authSvc.VerifyEmailToken(r.Context(), token)
	if err != nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/verify-email?error=invalid_token", http.StatusFound)
		return
	}

	// Issue new access token with email_verified=true
	accessToken, err := h.jwtManager.GenerateAccess(user.ID, user.Email, string(user.Plan), true)
	if err != nil {
		http.Redirect(w, r, h.cfg.AppURL+"/dashboard?verified=true", http.StatusFound)
		return
	}

	// Pass new token via redirect query param so SvelteKit can store it in memory
	http.Redirect(w, r, h.cfg.AppURL+"/auth/verify-email/success?token="+accessToken, http.StatusFound)
}

// MagicLink handles POST /api/v1/auth/magic-link
func (h *AuthHandler) MagicLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	// Always return 200 to avoid email enumeration — even if email doesn't exist yet
	_ = h.authSvc.SendMagicLink(r.Context(), req.Email)

	respondOK(w, map[string]string{"message": "magic link sent if email is valid"})
}

// MagicLinkVerify handles GET /api/v1/auth/magic-link/verify?token=...
func (h *AuthHandler) MagicLinkVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/magic-link?error=missing_token", http.StatusFound)
		return
	}

	user, err := h.authSvc.VerifyMagicLink(r.Context(), token)
	if err != nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/magic-link?error=invalid_or_expired", http.StatusFound)
		return
	}

	accessToken, err := h.jwtManager.GenerateAccess(user.ID, user.Email, string(user.Plan), user.EmailVerifiedAt != nil)
	if err != nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/login?error=token_error", http.StatusFound)
		return
	}

	refreshToken, err := h.jwtManager.GenerateRefresh(user.ID, h.cfg.JWTRefreshExpiryDay)
	if err != nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/login?error=token_error", http.StatusFound)
		return
	}

	h.setRefreshCookie(w, refreshToken)
	// Pass access token to SvelteKit via redirect — it will store in memory store
	http.Redirect(w, r, h.cfg.AppURL+"/auth/magic-link/success?token="+accessToken, http.StatusFound)
}

// GoogleRedirect handles GET /api/v1/auth/google
func (h *AuthHandler) GoogleRedirect(w http.ResponseWriter, r *http.Request) {
	if h.googleOAuth == nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/login?error=oauth_not_configured", http.StatusFound)
		return
	}

	// Generate random state (CSRF protection)
	state, err := generateOAuthState()
	if err != nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/login?error=server_error", http.StatusFound)
		return
	}

	// Store state in short-lived httpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteLaxMode, // Lax required for OAuth redirect flow
	})

	url := h.googleOAuth.AuthCodeURL(state, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusFound)
}

// GoogleCallback handles GET /api/v1/auth/google/callback
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if h.googleOAuth == nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/login?error=oauth_not_configured", http.StatusFound)
		return
	}

	// Validate state (CSRF)
	stateCookie, err := r.Cookie(oauthStateCookieName)
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/login?error=state_mismatch", http.StatusFound)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   oauthStateCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Exchange code for token
	oauthToken, err := h.googleOAuth.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/login?error=oauth_exchange_failed", http.StatusFound)
		return
	}

	// Fetch Google user info
	googleUser, err := fetchGoogleUserInfo(r.Context(), h.googleOAuth, oauthToken)
	if err != nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/login?error=userinfo_failed", http.StatusFound)
		return
	}

	// Upsert user in database
	user, err := h.authSvc.GetOrCreateByGoogle(r.Context(), googleUser.ID, googleUser.Email, googleUser.Name)
	if err != nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/login?error=server_error", http.StatusFound)
		return
	}

	accessToken, err := h.jwtManager.GenerateAccess(user.ID, user.Email, string(user.Plan), user.EmailVerifiedAt != nil)
	if err != nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/login?error=token_error", http.StatusFound)
		return
	}

	refreshToken, err := h.jwtManager.GenerateRefresh(user.ID, h.cfg.JWTRefreshExpiryDay)
	if err != nil {
		http.Redirect(w, r, h.cfg.AppURL+"/auth/login?error=token_error", http.StatusFound)
		return
	}

	h.setRefreshCookie(w, refreshToken)
	http.Redirect(w, r, h.cfg.AppURL+"/auth/callback?token="+accessToken, http.StatusFound)
}

// setRefreshCookie writes the refresh_token httpOnly cookie.
func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(time.Duration(h.cfg.JWTRefreshExpiryDay) * 24 * time.Hour),
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteStrictMode,
	})
}

// generateOAuthState creates a random state string for OAuth CSRF protection.
func generateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// googleUserInfo holds the parsed Google user info response.
type googleUserInfo struct {
	ID    string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// fetchGoogleUserInfo calls the Google UserInfo endpoint using the oauth2 token.
func fetchGoogleUserInfo(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*googleUserInfo, error) {
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("google userinfo: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("google userinfo: read body: %w", err)
	}

	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("google userinfo: unmarshal: %w", err)
	}
	if info.ID == "" || info.Email == "" {
		return nil, fmt.Errorf("google userinfo: missing id or email")
	}
	return &info, nil
}
