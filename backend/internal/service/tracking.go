package service

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TrackingService handles proposal view tracking.
type TrackingService struct {
	db     *pgxpool.Pool
	appURL string // base URL for dashboard links in emails
}

// NewTrackingService creates a new TrackingService.
func NewTrackingService(db *pgxpool.Pool, appURL string) *TrackingService {
	return &TrackingService{db: db, appURL: appURL}
}

// TrackOpen records an 'open' event for a proposal.
// Uses server-side tracking (called from SvelteKit SSR) to bypass ad blockers.
func (s *TrackingService) TrackOpen(ctx context.Context, proposalSlug, ip, userAgent, country string) error {
	// Resolve proposal ID and owner info
	var proposalID, ownerEmail, proposalTitle string
	var clientName *string
	err := s.db.QueryRow(ctx, `
		SELECT p.id, u.email, p.title, p.client_name
		FROM proposals p
		JOIN users u ON u.id = p.user_id
		WHERE p.slug = $1 AND p.slug_active = true AND p.deleted_at IS NULL
	`, proposalSlug).Scan(&proposalID, &ownerEmail, &proposalTitle, &clientName)
	if err != nil {
		return ErrNotFound
	}

	// Deduplication fingerprint: SHA-256(IP+UA), truncated to 16 hex chars (GDPR — no raw IP stored)
	fingerprint := fingerprintOf(ip, userAgent)

	// Check for recent duplicate (5-minute window — prevents F5 spam)
	var isDuplicate bool
	err = s.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM tracking_events
			WHERE proposal_id=$1 AND fingerprint=$2 AND created_at > NOW() - INTERVAL '5 minutes'
		)
	`, proposalID, fingerprint).Scan(&isDuplicate)
	if err != nil || isDuplicate {
		return nil // silently skip duplicates
	}

	// Insert tracking event and update proposal counters in a transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("tracking: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var countryVal *string
	if country != "" {
		countryVal = &country
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO tracking_events (proposal_id, event_type, country, user_agent, fingerprint)
		VALUES ($1, 'open', $2, $3, $4)
	`, proposalID, countryVal, userAgent, fingerprint)
	if err != nil {
		return fmt.Errorf("tracking: insert event: %w", err)
	}

	// Update proposal counters.
	// Status advances to 'opened' only when coming from 'sent' to avoid overwriting 'approved'.
	// first_opened_at is set only on the first ever open (COALESCE preserves existing value).
	_, err = tx.Exec(ctx, `
		UPDATE proposals SET
			open_count      = open_count + 1,
			last_opened_at  = NOW(),
			first_opened_at = COALESCE(first_opened_at, NOW()),
			status          = CASE WHEN status = 'sent' THEN 'opened' ELSE status END
		WHERE id = $1
	`, proposalID)
	if err != nil {
		return fmt.Errorf("tracking: update proposal: %w", err)
	}

	// Build dashboard link for the email
	proposalLink := fmt.Sprintf("%s/dashboard/proposals/%s", s.appURL, proposalID)

	// Use client_name in the email subject when available
	displayName := proposalTitle
	if clientName != nil && *clientName != "" {
		displayName = *clientName
	}

	// Enqueue first-open email notification — WHERE NOT EXISTS prevents duplicates
	_, err = tx.Exec(ctx, `
		INSERT INTO job_queue (job_type, payload)
		SELECT 'email_open_notify', jsonb_build_object(
			'proposal_id',    $1::text,
			'owner_email',    $2,
			'proposal_title', $3,
			'client_name',    $4,
			'proposal_link',  $5,
			'country',        $6::text
		)
		WHERE NOT EXISTS (
			SELECT 1 FROM job_queue
			WHERE job_type='email_open_notify'
			  AND payload->>'proposal_id' = $1::text
			  AND status IN ('pending', 'processing', 'done')
		)
	`, proposalID, ownerEmail, proposalTitle, displayName, proposalLink, country)
	if err != nil {
		return fmt.Errorf("tracking: enqueue email: %w", err)
	}

	return tx.Commit(ctx)
}

// TrackBlockTime records how long a client spent viewing a specific block.
func (s *TrackingService) TrackBlockTime(ctx context.Context, proposalSlug, blockID string, durationMs int) error {
	var proposalID string
	err := s.db.QueryRow(ctx, `
		SELECT id FROM proposals WHERE slug=$1 AND slug_active=true AND deleted_at IS NULL
	`, proposalSlug).Scan(&proposalID)
	if err != nil {
		return ErrNotFound
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO tracking_events (proposal_id, event_type, block_id, duration_ms)
		VALUES ($1, 'block_time', $2, $3)
	`, proposalID, blockID, durationMs)
	return err
}

// fingerprintOf creates a short fingerprint from IP and User-Agent.
// Only the first 16 hex characters are stored to avoid storing PII.
func fingerprintOf(ip, userAgent string) string {
	h := sha256.Sum256([]byte(ip + "|" + userAgent))
	return fmt.Sprintf("%x", h[:8]) // 16 hex chars = 64 bits
}
