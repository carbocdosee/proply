package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var hexColorRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// AccountService handles user profile, branding, and GDPR operations.
type AccountService struct {
	db         *pgxpool.Pool
	storageSvc *StorageService
	billingSvc *BillingService
}

// NewAccountService creates a new AccountService.
func NewAccountService(db *pgxpool.Pool, storageSvc *StorageService, billingSvc *BillingService) *AccountService {
	return &AccountService{db: db, storageSvc: storageSvc, billingSvc: billingSvc}
}

// UpdateProfileRequest carries optional profile fields.
// Nil fields are ignored (not updated).
type UpdateProfileRequest struct {
	Name     *string `json:"name"`
	Language *string `json:"language"`
}

// UpdateBrandingRequest carries optional branding fields.
// Nil fields are not updated.
// logo_url: nil = no change; pointer to "" = clear logo; pointer to URL = set logo.
type UpdateBrandingRequest struct {
	LogoURL          *string `json:"logo_url"`
	PrimaryColor     *string `json:"primary_color"`
	AccentColor      *string `json:"accent_color"`
	HideProplyFooter *bool   `json:"hide_proply_footer"`
}

// UpdateProfile saves name and/or language for the given user.
func (s *AccountService) UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) error {
	if req.Name == nil && req.Language == nil {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		UPDATE users
		SET
			name     = COALESCE($1, name),
			language = COALESCE($2, language)
		WHERE id = $3 AND deleted_at IS NULL
	`, req.Name, req.Language, userID)
	return err
}

// UpdateBranding saves logo_url, colors, and footer visibility for the given user.
// Returns ErrValidation if a color is not a valid #RRGGBB hex.
// Returns ErrPlanRequired if hide_proply_footer=true on a free plan.
func (s *AccountService) UpdateBranding(ctx context.Context, userID string, req UpdateBrandingRequest) error {
	if req.PrimaryColor != nil && !hexColorRegex.MatchString(*req.PrimaryColor) {
		return ErrValidation
	}
	if req.AccentColor != nil && !hexColorRegex.MatchString(*req.AccentColor) {
		return ErrValidation
	}

	// Plan gate: hiding the Proply footer requires Pro or Team.
	if req.HideProplyFooter != nil && *req.HideProplyFooter {
		var plan string
		_ = s.db.QueryRow(ctx,
			`SELECT plan FROM users WHERE id = $1 AND deleted_at IS NULL`, userID,
		).Scan(&plan)
		if plan == "free" {
			return ErrPlanRequired
		}
	}

	// logo_url update logic:
	//   $1 (logoProvided) = true  → update logo_url to NULLIF($2, '') so empty string clears it
	//   $1 (logoProvided) = false → keep current logo_url untouched
	_, err := s.db.Exec(ctx, `
		UPDATE users
		SET
			logo_url           = CASE WHEN $1 THEN NULLIF($2, '') ELSE logo_url END,
			primary_color      = COALESCE($3, primary_color),
			accent_color       = COALESCE($4, accent_color),
			hide_proply_footer = COALESCE($5, hide_proply_footer)
		WHERE id = $6 AND deleted_at IS NULL
	`, req.LogoURL != nil, req.LogoURL, req.PrimaryColor, req.AccentColor, req.HideProplyFooter, userID)
	return err
}

// UpdateRetention saves the data retention policy for a user.
// months must be one of 12, 24, or 36.
func (s *AccountService) UpdateRetention(ctx context.Context, userID string, months int) error {
	if months != 12 && months != 24 && months != 36 {
		return ErrValidation
	}
	_, err := s.db.Exec(ctx,
		`UPDATE users SET data_retention_months=$1 WHERE id=$2 AND deleted_at IS NULL`,
		months, userID,
	)
	return err
}

// ExportData returns a JSON snapshot of all user data (proposals + tracking events).
// Raw IP addresses are never included per GDPR requirements.
func (s *AccountService) ExportData(ctx context.Context, userID string) (json.RawMessage, error) {
	// User row
	var userEmail, userName, userPlan, userLanguage string
	var createdAt time.Time
	if err := s.db.QueryRow(ctx, `
		SELECT email, name, plan, language, created_at
		FROM users WHERE id=$1 AND deleted_at IS NULL
	`, userID).Scan(&userEmail, &userName, &userPlan, &userLanguage, &createdAt); err != nil {
		return nil, ErrNotFound
	}

	// Proposals (without blocks JSONB for brevity — export separately if needed)
	type exportProposal struct {
		ID           string     `json:"id"`
		Title        string     `json:"title"`
		ClientName   string     `json:"client_name"`
		Status       string     `json:"status"`
		Slug         *string    `json:"slug,omitempty"`
		OpenCount    int        `json:"open_count"`
		FirstOpenAt  *time.Time `json:"first_opened_at,omitempty"`
		LastOpenAt   *time.Time `json:"last_opened_at,omitempty"`
		ApprovedAt   *time.Time `json:"approved_at,omitempty"`
		CreatedAt    time.Time  `json:"created_at"`
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, title, client_name, status, slug,
		       open_count, first_opened_at, last_opened_at, approved_at, created_at
		FROM proposals WHERE user_id=$1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("export: proposals: %w", err)
	}
	defer rows.Close()

	exportProposals := make([]exportProposal, 0)
	var proposalIDs []string
	for rows.Next() {
		var p exportProposal
		if err := rows.Scan(
			&p.ID, &p.Title, &p.ClientName, &p.Status, &p.Slug,
			&p.OpenCount, &p.FirstOpenAt, &p.LastOpenAt, &p.ApprovedAt, &p.CreatedAt,
		); err != nil {
			continue
		}
		exportProposals = append(exportProposals, p)
		proposalIDs = append(proposalIDs, p.ID)
	}

	// Tracking events (no fingerprint / user_agent = PII, omit)
	type exportEvent struct {
		ProposalID string    `json:"proposal_id"`
		EventType  string    `json:"event_type"`
		Country    *string   `json:"country,omitempty"`
		CreatedAt  time.Time `json:"created_at"`
	}

	exportEvents := make([]exportEvent, 0)
	if len(proposalIDs) > 0 {
		eventRows, err := s.db.Query(ctx, `
			SELECT proposal_id, event_type, country, created_at
			FROM tracking_events
			WHERE proposal_id = ANY($1) AND event_type = 'open'
			ORDER BY created_at DESC
		`, proposalIDs)
		if err == nil {
			defer eventRows.Close()
			for eventRows.Next() {
				var e exportEvent
				if err := eventRows.Scan(&e.ProposalID, &e.EventType, &e.Country, &e.CreatedAt); err != nil {
					continue
				}
				exportEvents = append(exportEvents, e)
			}
		}
	}

	export := map[string]any{
		"exported_at": time.Now().UTC(),
		"user": map[string]any{
			"email":      userEmail,
			"name":       userName,
			"plan":       userPlan,
			"language":   userLanguage,
			"created_at": createdAt,
		},
		"proposals":       exportProposals,
		"tracking_events": exportEvents,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	return data, err
}

// DeleteAccount performs a full GDPR hard delete:
// 1. Cancels active Stripe subscription
// 2. Deletes all S3 media objects
// 3. Hard-deletes all DB rows (tracking_events cascade, proposals, subscriptions, users)
func (s *AccountService) DeleteAccount(ctx context.Context, userID string) error {
	// 1. Cancel Stripe subscription (best-effort, non-fatal)
	if s.billingSvc != nil {
		if err := s.billingSvc.CancelActiveSubscription(ctx, userID); err != nil {
			// Log but continue — user deletion should not be blocked by Stripe errors
			_ = err
		}
	}

	// 2. Collect proposal IDs for S3 cleanup
	rows, err := s.db.Query(ctx, `SELECT id FROM proposals WHERE user_id=$1`, userID)
	if err != nil {
		return fmt.Errorf("account delete: list proposals: %w", err)
	}
	var proposalIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			proposalIDs = append(proposalIDs, id)
		}
	}
	rows.Close()

	// 3. Delete S3 objects (best-effort)
	if s.storageSvc != nil {
		_ = s.storageSvc.DeleteUserObjects(ctx, userID, proposalIDs)
	}

	// 4. Hard-delete from database (tracking_events cascade from proposals)
	_, err = s.db.Exec(ctx, `DELETE FROM proposals WHERE user_id=$1`, userID)
	if err != nil {
		return fmt.Errorf("account delete: proposals: %w", err)
	}

	_, err = s.db.Exec(ctx, `DELETE FROM subscriptions WHERE user_id=$1`, userID)
	if err != nil {
		return fmt.Errorf("account delete: subscriptions: %w", err)
	}

	_, err = s.db.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	return err
}

// DeleteExpiredProposals hard-deletes proposals that have exceeded the owner's
// data_retention_months setting. Called from the background worker daily.
func (s *AccountService) DeleteExpiredProposals(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM proposals
		WHERE deleted_at IS NOT NULL
		   OR created_at < NOW() - (
			   SELECT (data_retention_months || ' months')::interval
			   FROM users
			   WHERE users.id = proposals.user_id
		   )
	`)
	return err
}
