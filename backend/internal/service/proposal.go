package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
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
	Plan    string
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

// List returns paginated proposals for a user with optional filtering, search, and sorting.
func (s *ProposalService) List(ctx context.Context, f ProposalFilter) (*ProposalListResult, error) {
	// Build base conditions
	args := []any{f.UserID}
	where := `WHERE p.user_id = $1 AND p.deleted_at IS NULL`

	if f.Status != "" && f.Status != "all" {
		args = append(args, f.Status)
		where += fmt.Sprintf(` AND p.status = $%d`, len(args))
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		idx := fmt.Sprintf("$%d", len(args))
		where += fmt.Sprintf(` AND (p.title ILIKE %s OR p.client_name ILIKE %s)`, idx, idx)
	}

	// Sorting
	orderCol := `p.updated_at`
	switch f.Sort {
	case "created_at":
		orderCol = `p.created_at`
	case "last_opened_at":
		orderCol = `p.last_opened_at`
	case "title":
		orderCol = `p.title`
	}
	order := orderCol
	if f.Order == "asc" {
		order += " ASC"
	} else {
		order += " DESC"
	}

	// Count total
	var total int
	countQuery := `SELECT COUNT(*) FROM proposals p ` + where
	if err := s.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("proposal: list count: %w", err)
	}

	// Count active (used for plan_usage)
	var usedCount int
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM proposals
		WHERE user_id = $1 AND status IN ('sent','opened','approved') AND deleted_at IS NULL
	`, f.UserID).Scan(&usedCount); err != nil {
		return nil, fmt.Errorf("proposal: list plan usage: %w", err)
	}

	// Paginated items
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 20
	}
	offset := (f.Page - 1) * f.PerPage
	args = append(args, f.PerPage, offset)
	dataQuery := fmt.Sprintf(`
		SELECT p.id, p.title, p.client_name, p.status, p.slug, p.slug_active,
		       p.open_count, p.first_opened_at, p.last_opened_at, p.approved_at,
		       p.created_at, p.updated_at
		FROM proposals p
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, order, len(args)-1, len(args))

	rows, err := s.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("proposal: list query: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Proposal, 0)
	for rows.Next() {
		var p domain.Proposal
		if err := rows.Scan(
			&p.ID, &p.Title, &p.ClientName, &p.Status, &p.Slug, &p.SlugActive,
			&p.OpenCount, &p.FirstOpenedAt, &p.LastOpenedAt, &p.ApprovedAt,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("proposal: list scan: %w", err)
		}
		p.Blocks = []domain.Block{} // not needed in list view
		items = append(items, p)
	}

	result := &ProposalListResult{
		Items:   items,
		Total:   total,
		Page:    f.Page,
		PerPage: f.PerPage,
	}
	result.PlanUsage.Used = usedCount
	if f.Plan == string(domain.PlanFree) {
		limit := 3
		result.PlanUsage.Limit = &limit
	}
	return result, nil
}

// Create creates a new draft proposal, optionally pre-filled from a template.
// Free-plan users are limited to 3 total (non-deleted) proposals.
func (s *ProposalService) Create(ctx context.Context, userID, plan, title, clientName string, templateID *string) (*domain.Proposal, error) {
	if title == "" {
		title = "Untitled"
	}

	// Plan gate: Free users limited to 3 proposals total
	if domain.Plan(plan) == domain.PlanFree {
		var count int
		if err := s.db.QueryRow(ctx, `
			SELECT COUNT(*) FROM proposals WHERE user_id=$1 AND deleted_at IS NULL
		`, userID).Scan(&count); err != nil {
			return nil, fmt.Errorf("proposal: create: count check: %w", err)
		}
		if count >= 3 {
			return nil, ErrPlanLimit
		}
	}

	// Resolve template blocks
	blocks := []domain.Block{}
	if templateID != nil && *templateID != "" {
		tplBlocks, err := blocksForTemplate(*templateID)
		if err != nil {
			return nil, fmt.Errorf("proposal: create: build template: %w", err)
		}
		if tplBlocks != nil {
			blocks = tplBlocks
		}
	}

	blocksJSON, err := json.Marshal(blocks)
	if err != nil {
		return nil, fmt.Errorf("proposal: create: marshal blocks: %w", err)
	}

	var proposal domain.Proposal
	err = s.db.QueryRow(ctx, `
		INSERT INTO proposals (user_id, title, client_name, template_id, blocks)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, title, client_name, status, created_at, updated_at
	`, userID, title, clientName, templateID, blocksJSON).Scan(
		&proposal.ID, &proposal.Title, &proposal.ClientName,
		&proposal.Status, &proposal.CreatedAt, &proposal.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("proposal: create: %w", err)
	}
	proposal.Blocks = blocks
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
// Returns ErrRevoked if the slug exists but has been deactivated.
// Returns ErrNotFound if the slug does not exist at all.
func (s *ProposalService) GetPublic(ctx context.Context, proposalSlug string) (*domain.PublicProposal, error) {
	// First check if the slug exists (active or not) to differentiate revoked vs unknown.
	var slugActive bool
	slugCheckErr := s.db.QueryRow(ctx,
		`SELECT slug_active FROM proposals WHERE slug = $1 AND deleted_at IS NULL`,
		proposalSlug,
	).Scan(&slugActive)
	if slugCheckErr != nil {
		return nil, ErrNotFound
	}
	if !slugActive {
		return nil, ErrRevoked
	}

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
	// Validate email format before hitting the database
	if _, err := mail.ParseAddress(clientEmail); err != nil {
		return nil, ErrValidation
	}

	// Fetch proposal + owner data needed for email notifications
	var proposalID, proposalTitle, ownerEmail, agencyName string
	var status domain.ProposalStatus
	err := s.db.QueryRow(ctx, `
		SELECT p.id, p.title, p.status, u.email, u.name
		FROM proposals p
		JOIN users u ON u.id = p.user_id
		WHERE p.slug=$1 AND p.slug_active=true AND p.deleted_at IS NULL
	`, proposalSlug).Scan(&proposalID, &proposalTitle, &status, &ownerEmail, &agencyName)
	if err != nil {
		return nil, ErrNotFound
	}
	if status == domain.StatusApproved {
		return nil, ErrConflict
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("proposal: approve: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var approvedAt time.Time
	err = tx.QueryRow(ctx, `
		UPDATE proposals SET status='approved', client_email=$1, approved_at=NOW(), updated_at=NOW()
		WHERE id=$2 RETURNING approved_at
	`, clientEmail, proposalID).Scan(&approvedAt)
	if err != nil {
		return nil, fmt.Errorf("proposal: approve: update: %w", err)
	}

	approvedAtStr := approvedAt.Format(time.RFC3339)

	// Build properly encoded JSON payloads for both notification jobs
	ownerPayload, _ := json.Marshal(map[string]string{
		"owner_email":    ownerEmail,
		"proposal_title": proposalTitle,
		"client_email":   clientEmail,
		"approved_at":    approvedAtStr,
	})
	clientPayload, _ := json.Marshal(map[string]string{
		"client_email":   clientEmail,
		"agency_name":    agencyName,
		"proposal_title": proposalTitle,
		"approved_at":    approvedAtStr,
	})

	_, err = tx.Exec(ctx, `
		INSERT INTO job_queue (job_type, payload) VALUES
			('email_approved_notify', $1),
			('email_client_approved', $2)
	`, ownerPayload, clientPayload)
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

	isPro := domain.Plan(plan) != domain.PlanFree

	result := &AnalyticsResult{
		BlockStats: []BlockStat{},
		Events:     []domain.TrackingEvent{},
		PlanGate:   !isPro,
	}

	// Query denormalized counters from the proposals row (fast)
	var totalDurationMs int64
	err := s.db.QueryRow(ctx, `
		SELECT
			COALESCE(open_count, 0),
			first_opened_at,
			last_opened_at
		FROM proposals WHERE id = $1
	`, id).Scan(&result.OpenCount, &result.FirstOpenedAt, &result.LastOpenedAt)
	if err != nil {
		return nil, fmt.Errorf("analytics: proposal stats: %w", err)
	}

	// Total time spent across all block_time events
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(duration_ms), 0)
		FROM tracking_events
		WHERE proposal_id = $1 AND event_type = 'block_time'
	`, id).Scan(&totalDurationMs)
	result.TotalDurationSec = int(totalDurationMs / 1000)

	// Per-block stats (Pro only): aggregate duration per block, join JSONB for type/order
	if isPro {
		rows, err := s.db.Query(ctx, `
			SELECT
				te.block_id::text,
				COALESCE(blk.value->>'type', 'unknown') AS block_type,
				COALESCE((blk.value->>'order')::int, 0)  AS block_order,
				SUM(te.duration_ms)                      AS total_ms
			FROM tracking_events te
			LEFT JOIN LATERAL (
				SELECT elem AS value
				FROM jsonb_array_elements(
					(SELECT blocks FROM proposals WHERE id = $1)
				) AS elem
				WHERE elem->>'id' = te.block_id::text
			) blk ON true
			WHERE te.proposal_id = $1
			  AND te.event_type  = 'block_time'
			  AND te.block_id IS NOT NULL
			GROUP BY te.block_id, blk.value->>'type', blk.value->>'order'
			ORDER BY block_order ASC
		`, id)
		if err != nil {
			return nil, fmt.Errorf("analytics: block stats: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var bs BlockStat
			var totalMs int64
			if err := rows.Scan(&bs.BlockID, &bs.BlockType, &bs.Order, &totalMs); err != nil {
				continue
			}
			bs.DurationSec = int(totalMs / 1000)
			result.BlockStats = append(result.BlockStats, bs)
		}
	}

	return result, nil
}
