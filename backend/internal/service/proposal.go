package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"proply/internal/domain"
	"proply/pkg/slug"
)

// ProposalService handles proposal business logic.
type ProposalService struct {
	db *pgxpool.Pool
}

// NewProposalService creates a new ProposalService.
func NewProposalService(db *pgxpool.Pool) *ProposalService {
	return &ProposalService{db: db}
}

// ProposalFilter holds query parameters for listing proposals.
type ProposalFilter struct {
	UserID  string
	Status  string
	Search  string
	Sort    string
	Order   string
	Page    int
	PerPage int
}

// ProposalListResult is the paginated response for listing proposals.
type ProposalListResult struct {
	Items    []domain.Proposal `json:"items"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PerPage  int               `json:"per_page"`
	PlanUsage struct {
		Used  int  `json:"used"`
		Limit *int `json:"limit"` // nil for Pro
	} `json:"plan_usage"`
}

// AnalyticsResult holds the analytics data for a single proposal.
type AnalyticsResult struct {
	OpenCount      int                    `json:"open_count"`
	FirstOpenedAt  *time.Time             `json:"first_opened_at,omitempty"`
	LastOpenedAt   *time.Time             `json:"last_opened_at,omitempty"`
	TotalDurationSec int                  `json:"total_duration_sec"`
	BlockStats     []BlockStat            `json:"block_stats"`
	Events         []domain.TrackingEvent `json:"events"`
	PlanGate       bool                   `json:"plan_gate"`
}

// BlockStat holds per-block analytics.
type BlockStat struct {
	BlockID     string `json:"block_id"`
	BlockType   string `json:"block_type"`
	Order       int    `json:"order"`
	DurationSec int    `json:"duration_sec"`
}

// List returns paginated proposals for a user.
func (s *ProposalService) List(ctx context.Context, f ProposalFilter) (*ProposalListResult, error) {
	// TODO: implement full query with filters, sorting, pagination
	return &ProposalListResult{Items: []domain.Proposal{}, Total: 0, Page: f.Page, PerPage: f.PerPage}, nil
}

// Create creates a new draft proposal.
func (s *ProposalService) Create(ctx context.Context, userID, plan, title, clientName string, templateID *string) (*domain.Proposal, error) {
	if title == "" {
		title = "Без названия"
	}

	var proposal domain.Proposal
	err := s.db.QueryRow(ctx, `
		INSERT INTO proposals (user_id, title, client_name, template_id, blocks)
		VALUES ($1, $2, $3, $4, '[]')
		RETURNING id, title, client_name, status, created_at, updated_at
	`, userID, title, clientName, templateID).Scan(
		&proposal.ID, &proposal.Title, &proposal.ClientName,
		&proposal.Status, &proposal.CreatedAt, &proposal.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("proposal: create: %w", err)
	}
	return &proposal, nil
}

// GetByID fetches a proposal by ID for the authenticated owner.
func (s *ProposalService) GetByID(ctx context.Context, id, userID string) (*domain.Proposal, error) {
	var p domain.Proposal
	var blocksJSON []byte

	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, title, client_name, client_email, status, slug, slug_active,
		       blocks, template_id, first_opened_at, last_opened_at, open_count,
		       approved_at, created_at, updated_at
		FROM proposals
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&p.ID, &p.UserID, &p.Title, &p.ClientName, &p.ClientEmail,
		&p.Status, &p.Slug, &p.SlugActive, &blocksJSON,
		&p.TemplateID, &p.FirstOpenedAt, &p.LastOpenedAt, &p.OpenCount,
		&p.ApprovedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, ErrNotFound
	}
	if p.UserID != userID {
		return nil, ErrForbidden
	}
	if err := json.Unmarshal(blocksJSON, &p.Blocks); err != nil {
		p.Blocks = []domain.Block{}
	}
	return &p, nil
}

// Update applies partial updates to a proposal.
func (s *ProposalService) Update(ctx context.Context, id, userID string, title, clientName *string, blocks []domain.Block) (*time.Time, error) {
	p, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if p.Status == domain.StatusApproved {
		return nil, ErrConflict
	}

	if title != nil {
		p.Title = *title
	}
	if clientName != nil {
		p.ClientName = *clientName
	}
	if blocks != nil {
		p.Blocks = blocks
	}

	blocksJSON, err := json.Marshal(p.Blocks)
	if err != nil {
		return nil, fmt.Errorf("proposal: marshal blocks: %w", err)
	}

	var updatedAt time.Time
	err = s.db.QueryRow(ctx, `
		UPDATE proposals SET title=$1, client_name=$2, blocks=$3, updated_at=NOW()
		WHERE id=$4 AND deleted_at IS NULL
		RETURNING updated_at
	`, p.Title, p.ClientName, blocksJSON, id).Scan(&updatedAt)
	if err != nil {
		return nil, fmt.Errorf("proposal: update: %w", err)
	}
	return &updatedAt, nil
}

// Publish generates a slug and transitions proposal to 'sent' status.
func (s *ProposalService) Publish(ctx context.Context, id, userID, plan string) (string, error) {
	p, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return "", err
	}
	if p.Slug != nil && *p.Slug != "" {
		return "", ErrConflict
	}

	// Check plan limit for Free users
	if domain.Plan(plan) == domain.PlanFree {
		var count int
		err := s.db.QueryRow(ctx, `
			SELECT COUNT(*) FROM proposals
			WHERE user_id=$1 AND status IN ('sent','opened','approved') AND deleted_at IS NULL
		`, userID).Scan(&count)
		if err == nil && count >= 3 {
			return "", ErrPlanLimit
		}
	}

	db := s.db
	newSlug, err := slug.GenerateWithRetry(func(candidate string) (bool, error) {
		var exists bool
		qErr := db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM proposals WHERE slug=$1)`, candidate,
		).Scan(&exists)
		return exists, qErr
	})
	if err != nil {
		return "", fmt.Errorf("proposal: generate slug: %w", err)
	}

	_, err = s.db.Exec(ctx, `
		UPDATE proposals SET slug=$1, slug_active=true, status='sent', updated_at=NOW()
		WHERE id=$2
	`, newSlug, id)
	if err != nil {
		return "", fmt.Errorf("proposal: publish: %w", err)
	}

	return newSlug, nil
}

// UpdateStatus applies a manual status transition.
func (s *ProposalService) UpdateStatus(ctx context.Context, id, userID, status string) (string, error) {
	p, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return "", err
	}
	// Approved is a terminal state — cannot be changed by owner
	if p.Status == domain.StatusApproved {
		return "", ErrConflict
	}

	_, err = s.db.Exec(ctx, `
		UPDATE proposals SET status=$1, updated_at=NOW() WHERE id=$2
	`, status, id)
	if err != nil {
		return "", fmt.Errorf("proposal: update status: %w", err)
	}
	return status, nil
}

// Revoke deactivates a published proposal link.
func (s *ProposalService) Revoke(ctx context.Context, id, userID string) error {
	if _, err := s.GetByID(ctx, id, userID); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `UPDATE proposals SET slug_active=false, updated_at=NOW() WHERE id=$1`, id)
	return err
}

// Duplicate creates a copy of an existing proposal.
func (s *ProposalService) Duplicate(ctx context.Context, id, userID string) (string, error) {
	p, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return "", err
	}

	blocksJSON, _ := json.Marshal(p.Blocks)
	var newID string
	err = s.db.QueryRow(ctx, `
		INSERT INTO proposals (user_id, title, client_name, blocks, template_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, userID, p.Title+" (копия)", p.ClientName, blocksJSON, p.TemplateID).Scan(&newID)
	if err != nil {
		return "", fmt.Errorf("proposal: duplicate: %w", err)
	}
	return newID, nil
}

// Delete soft-deletes a proposal.
func (s *ProposalService) Delete(ctx context.Context, id, userID string) error {
	if _, err := s.GetByID(ctx, id, userID); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `UPDATE proposals SET deleted_at=NOW() WHERE id=$1`, id)
	return err
}

// GetPublic returns the proposal data for the public viewer.
func (s *ProposalService) GetPublic(ctx context.Context, proposalSlug string) (*domain.PublicProposal, error) {
	var p domain.PublicProposal
	var blocksJSON []byte
	var passwordHash *string

	err := s.db.QueryRow(ctx, `
		SELECT p.title, p.client_name, u.name, u.logo_url, u.primary_color, u.accent_color,
		       u.hide_proply_footer, u.language, p.blocks, p.status, p.approved_at, p.password_hash
		FROM proposals p
		JOIN users u ON u.id = p.user_id
		WHERE p.slug = $1 AND p.slug_active = true AND p.deleted_at IS NULL
	`, proposalSlug).Scan(
		&p.Title, &p.ClientName, &p.AgencyName, &p.LogoURL,
		&p.PrimaryColor, &p.AccentColor, &p.HideProplyFooter, &p.Language,
		&blocksJSON, &p.Status, &p.ApprovedAt, &passwordHash,
	)
	if err != nil {
		return nil, ErrNotFound
	}

	p.PasswordProtected = passwordHash != nil
	if err := json.Unmarshal(blocksJSON, &p.Blocks); err != nil {
		p.Blocks = []domain.Block{}
	}
	return &p, nil
}

// VerifyProposalPassword checks if the provided password matches the proposal's password hash.
func (s *ProposalService) VerifyProposalPassword(ctx context.Context, proposalSlug, password string) error {
	var hash *string
	err := s.db.QueryRow(ctx, `SELECT password_hash FROM proposals WHERE slug=$1`, proposalSlug).Scan(&hash)
	if err != nil || hash == nil {
		return ErrNotFound
	}
	// TODO: implement bcrypt comparison when password protection feature is enabled
	// bcrypt.CompareHashAndPassword([]byte(*hash), []byte(password))
	_ = password
	return nil
}

// Approve handles the client approval flow.
func (s *ProposalService) Approve(ctx context.Context, proposalSlug, clientEmail string) (*time.Time, error) {
	var proposalID string
	var status domain.ProposalStatus
	err := s.db.QueryRow(ctx, `
		SELECT id, status FROM proposals
		WHERE slug=$1 AND slug_active=true AND deleted_at IS NULL
	`, proposalSlug).Scan(&proposalID, &status)
	if err != nil {
		return nil, ErrNotFound
	}
	if status == domain.StatusApproved {
		return nil, ErrConflict
	}

	var approvedAt time.Time
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("proposal: approve: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		UPDATE proposals SET status='approved', client_email=$1, approved_at=NOW(), updated_at=NOW()
		WHERE id=$2 RETURNING approved_at
	`, clientEmail, proposalID).Scan(&approvedAt)
	if err != nil {
		return nil, fmt.Errorf("proposal: approve: update: %w", err)
	}

	// Enqueue email notifications
	_, err = tx.Exec(ctx, `
		INSERT INTO job_queue (job_type, payload)
		VALUES ('email_approved_notify', $1), ('email_client_approved', $2)
	`,
		fmt.Sprintf(`{"proposal_id":"%s","client_email":"%s"}`, proposalID, clientEmail),
		fmt.Sprintf(`{"proposal_id":"%s","client_email":"%s"}`, proposalID, clientEmail),
	)
	if err != nil {
		return nil, fmt.Errorf("proposal: approve: enqueue jobs: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("proposal: approve: commit: %w", err)
	}

	return &approvedAt, nil
}

// GetAnalytics returns analytics data for a proposal.
func (s *ProposalService) GetAnalytics(ctx context.Context, id, userID, plan string) (*AnalyticsResult, error) {
	if _, err := s.GetByID(ctx, id, userID); err != nil {
		return nil, err
	}

	result := &AnalyticsResult{
		BlockStats: []BlockStat{},
		Events:     []domain.TrackingEvent{},
		PlanGate:   domain.Plan(plan) == domain.PlanFree,
	}

	// TODO: query tracking_events for full analytics
	return result, nil
}
