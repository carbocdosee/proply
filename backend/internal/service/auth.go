package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"proply/internal/domain"
)

const (
	magicLinkTTL        = 15 * time.Minute
	emailVerificationTTL = 24 * time.Hour
	tokenBytes           = 32 // 256-bit random token
)

// AuthService handles user registration, login, and profile retrieval.
type AuthService struct {
	db  *pgxpool.Pool
	cfg authConfig
}

// authConfig holds auth-specific config values injected at construction.
type authConfig struct {
	AppURL        string
	EmailFromAddr string
	EmailFromName string
}

// NewAuthService creates a new AuthService.
func NewAuthService(db *pgxpool.Pool, appURL, emailFromAddr, emailFromName string) *AuthService {
	return &AuthService{
		db: db,
		cfg: authConfig{
			AppURL:        appURL,
			EmailFromAddr: emailFromAddr,
			EmailFromName: emailFromName,
		},
	}
}

// Register creates a new user account and enqueues a verification email.
func (s *AuthService) Register(ctx context.Context, email, password, name string) (*domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)

	if _, err := mail.ParseAddress(email); err != nil {
		return nil, ErrValidation
	}
	if len(password) < 8 {
		return nil, ErrValidation
	}
	if name == "" {
		return nil, ErrValidation
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("auth: hash password: %w", err)
	}

	var user domain.User
	err = s.db.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, plan, language, primary_color, accent_color,
		          hide_proply_footer, data_retention_months, email_verified_at, created_at
	`, email, name, string(hash)).Scan(
		&user.ID, &user.Email, &user.Name, &user.Plan, &user.Language,
		&user.PrimaryColor, &user.AccentColor, &user.HideProplyFooter,
		&user.DataRetentionMonths, &user.EmailVerifiedAt, &user.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailExists
		}
		return nil, fmt.Errorf("auth: register: %w", err)
	}

	// Enqueue verification email (non-blocking: ignore enqueue errors)
	_ = s.enqueueEmailVerification(ctx, user.ID, email)

	return &user, nil
}

// Login authenticates a user by email and password.
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	var user domain.User
	var passwordHash *string
	err := s.db.QueryRow(ctx, `
		SELECT id, email, name, password_hash, plan, language, primary_color, accent_color,
		       email_verified_at, created_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`, email).Scan(
		&user.ID, &user.Email, &user.Name, &passwordHash,
		&user.Plan, &user.Language, &user.PrimaryColor, &user.AccentColor,
		&user.EmailVerifiedAt, &user.CreatedAt,
	)
	if err != nil {
		// Return generic error to avoid email enumeration
		return nil, fmt.Errorf("auth: invalid credentials")
	}

	if passwordHash == nil {
		// User registered via OAuth — no password set
		return nil, fmt.Errorf("auth: no password (use Google or magic link)")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*passwordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("auth: invalid credentials")
	}

	return &user, nil
}

// GetByID fetches a user by ID.
func (s *AuthService) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	err := s.db.QueryRow(ctx, `
		SELECT id, email, name, plan, language, logo_url, primary_color, accent_color,
		       hide_proply_footer, data_retention_months, email_verified_at, created_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.Plan, &user.Language,
		&user.LogoURL, &user.PrimaryColor, &user.AccentColor,
		&user.HideProplyFooter, &user.DataRetentionMonths,
		&user.EmailVerifiedAt, &user.CreatedAt,
	)
	if err != nil {
		return nil, ErrNotFound
	}
	return &user, nil
}

// GetOrCreateByGoogle finds an existing user by Google ID (or email), or creates a new verified user.
// Called from the Google OAuth callback handler.
func (s *AuthService) GetOrCreateByGoogle(ctx context.Context, googleID, email, name string) (*domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	now := time.Now()

	// Try to find by google_id first, then fall back to email
	var user domain.User
	err := s.db.QueryRow(ctx, `
		SELECT id, email, name, plan, language, primary_color, accent_color,
		       hide_proply_footer, data_retention_months, email_verified_at, created_at
		FROM users
		WHERE (google_id = $1 OR email = $2) AND deleted_at IS NULL
		LIMIT 1
	`, googleID, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.Plan, &user.Language,
		&user.PrimaryColor, &user.AccentColor, &user.HideProplyFooter,
		&user.DataRetentionMonths, &user.EmailVerifiedAt, &user.CreatedAt,
	)

	if err == nil {
		// User found: update google_id and mark email as verified (OAuth = verified email)
		_, _ = s.db.Exec(ctx, `
			UPDATE users SET google_id = $1, email_verified_at = COALESCE(email_verified_at, $2)
			WHERE id = $3
		`, googleID, now, user.ID)
		if user.EmailVerifiedAt == nil {
			user.EmailVerifiedAt = &now
		}
		return &user, nil
	}

	// Create new user — Google OAuth = email already verified
	err = s.db.QueryRow(ctx, `
		INSERT INTO users (email, name, google_id, email_verified_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, name, plan, language, primary_color, accent_color,
		          hide_proply_footer, data_retention_months, email_verified_at, created_at
	`, email, name, googleID, now).Scan(
		&user.ID, &user.Email, &user.Name, &user.Plan, &user.Language,
		&user.PrimaryColor, &user.AccentColor, &user.HideProplyFooter,
		&user.DataRetentionMonths, &user.EmailVerifiedAt, &user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("auth: google upsert: %w", err)
	}
	return &user, nil
}

// SendMagicLink generates a one-time login token and enqueues a magic link email.
// Works for both existing and new users (creates unverified account if needed).
func (s *AuthService) SendMagicLink(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrValidation
	}

	// Find or lazily create the user
	var userID string
	err := s.db.QueryRow(ctx, `
		SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL
	`, email).Scan(&userID)
	if err != nil {
		// Create a minimal user record so the token has a user_id to link to
		err = s.db.QueryRow(ctx, `
			INSERT INTO users (email, name) VALUES ($1, $2)
			ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
			RETURNING id
		`, email, email).Scan(&userID)
		if err != nil {
			return fmt.Errorf("auth: magic link upsert user: %w", err)
		}
	}

	raw, tokenHash, err := generateToken()
	if err != nil {
		return fmt.Errorf("auth: generate magic link token: %w", err)
	}

	// Invalidate any prior unused magic link tokens for this user
	_, _ = s.db.Exec(ctx, `
		UPDATE auth_tokens SET used_at = NOW()
		WHERE user_id = $1 AND type = 'magic_link' AND used_at IS NULL
	`, userID)

	_, err = s.db.Exec(ctx, `
		INSERT INTO auth_tokens (token_hash, type, user_id, email, expires_at)
		VALUES ($1, 'magic_link', $2, $3, $4)
	`, tokenHash, userID, email, time.Now().Add(magicLinkTTL))
	if err != nil {
		return fmt.Errorf("auth: store magic link token: %w", err)
	}

	// Enqueue email delivery
	link := s.cfg.AppURL + "/auth/magic-link/verify?token=" + raw
	payload, _ := json.Marshal(map[string]string{
		"to":    email,
		"link":  link,
		"email": email,
	})
	_, err = s.db.Exec(ctx, `
		INSERT INTO job_queue (job_type, payload) VALUES ('email_magic_link', $1)
	`, payload)
	return err
}

// VerifyMagicLink validates a magic link token and returns the authenticated user.
func (s *AuthService) VerifyMagicLink(ctx context.Context, rawToken string) (*domain.User, error) {
	tokenHash := hashToken(rawToken)

	var tokenID string
	var userID string
	var expiresAt time.Time
	var usedAt *time.Time

	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, expires_at, used_at
		FROM auth_tokens
		WHERE token_hash = $1 AND type = 'magic_link'
	`, tokenHash).Scan(&tokenID, &userID, &expiresAt, &usedAt)
	if err != nil {
		return nil, ErrNotFound
	}
	if usedAt != nil {
		return nil, fmt.Errorf("auth: magic link already used")
	}
	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("auth: magic link expired")
	}

	// Mark token as used
	_, _ = s.db.Exec(ctx, `UPDATE auth_tokens SET used_at = NOW() WHERE id = $1`, tokenID)

	// Mark email as verified (magic link = confirmed email ownership)
	now := time.Now()
	_, _ = s.db.Exec(ctx, `
		UPDATE users SET email_verified_at = COALESCE(email_verified_at, $1) WHERE id = $2
	`, now, userID)

	return s.GetByID(ctx, userID)
}

// SendVerificationEmail enqueues an email verification token for the given user.
func (s *AuthService) SendVerificationEmail(ctx context.Context, userID, email string) error {
	return s.enqueueEmailVerification(ctx, userID, email)
}

// VerifyEmailToken validates an email verification token and marks the user's email as verified.
func (s *AuthService) VerifyEmailToken(ctx context.Context, rawToken string) (*domain.User, error) {
	tokenHash := hashToken(rawToken)

	var tokenID, userID string
	var expiresAt time.Time
	var usedAt *time.Time

	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, expires_at, used_at
		FROM auth_tokens
		WHERE token_hash = $1 AND type = 'email_verification'
	`, tokenHash).Scan(&tokenID, &userID, &expiresAt, &usedAt)
	if err != nil {
		return nil, ErrNotFound
	}
	if usedAt != nil {
		// Already verified — still return the user so frontend can redirect to dashboard
		return s.GetByID(ctx, userID)
	}
	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("auth: verification link expired")
	}

	_, _ = s.db.Exec(ctx, `UPDATE auth_tokens SET used_at = NOW() WHERE id = $1`, tokenID)
	_, _ = s.db.Exec(ctx, `
		UPDATE users SET email_verified_at = NOW() WHERE id = $1 AND email_verified_at IS NULL
	`, userID)

	return s.GetByID(ctx, userID)
}

// enqueueEmailVerification creates a verification token and queues the email job.
func (s *AuthService) enqueueEmailVerification(ctx context.Context, userID, email string) error {
	raw, tokenHash, err := generateToken()
	if err != nil {
		return fmt.Errorf("auth: generate verification token: %w", err)
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO auth_tokens (token_hash, type, user_id, email, expires_at)
		VALUES ($1, 'email_verification', $2, $3, $4)
	`, tokenHash, userID, email, time.Now().Add(emailVerificationTTL))
	if err != nil {
		return fmt.Errorf("auth: store verification token: %w", err)
	}

	link := s.cfg.AppURL + "/auth/verify-email?token=" + raw
	payload, _ := json.Marshal(map[string]string{
		"to":    email,
		"link":  link,
		"email": email,
	})
	_, err = s.db.Exec(ctx, `
		INSERT INTO job_queue (job_type, payload) VALUES ('email_verification', $1)
	`, payload)
	return err
}

// generateToken creates a cryptographically random token.
// Returns (rawToken, tokenHash, error). Only rawToken is sent to the user.
func generateToken() (raw, hash string, err error) {
	b := make([]byte, tokenBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	hash = hashToken(raw)
	return raw, hash, nil
}

// hashToken hashes a raw token with SHA-256 for safe DB storage.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h[:])
}

// isUniqueViolation returns true if the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
