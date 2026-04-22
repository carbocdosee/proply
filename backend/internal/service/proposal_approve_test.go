package service

// TASK-AQA-108 — Proposal Approval: unit + integration tests
//
// AC coverage:
//   AC2 / AC3  — email validation (unit, no DB): empty / no-@ email → ErrValidation
//   AC5        — POST /approve → status='approved', client_email saved, approved_at set  (integration)
//   AC6        — email job enqueued for client (email_client_approved)                   (integration)
//   AC7        — email job enqueued for owner  (email_approved_notify)                   (integration)
//   AC9        — second approve call → ErrConflict                                        (integration)
//
// AC1, AC4, AC8 are covered at the E2E layer (proply/frontend/e2e/approval.spec.ts).
//
// Integration tests require TEST_DATABASE_URL. Run with:
//   TEST_DATABASE_URL=postgres://... go test ./internal/service/... -run TestApprove

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// ─── Unit: email validation (no DB required) ─────────────────────────────────

// AC2: empty email → ErrValidation before any DB call.
func TestApprove_EmptyEmail_ReturnsValidationError(t *testing.T) {
	svc := &ProposalService{db: nil} // email gate fires before DB access
	_, err := svc.Approve(context.Background(), "any-slug", "")
	if err != ErrValidation {
		t.Errorf("want ErrValidation for empty email, got %v", err)
	}
}

// AC3: email without @ → ErrValidation.
func TestApprove_NoAtEmail_ReturnsValidationError(t *testing.T) {
	svc := &ProposalService{db: nil}
	_, err := svc.Approve(context.Background(), "any-slug", "notanemail")
	if err != ErrValidation {
		t.Errorf("want ErrValidation for 'notanemail', got %v", err)
	}
}

// Additional edge: whitespace-only email → ErrValidation.
func TestApprove_WhitespaceEmail_ReturnsValidationError(t *testing.T) {
	svc := &ProposalService{db: nil}
	_, err := svc.Approve(context.Background(), "any-slug", "   ")
	if err != ErrValidation {
		t.Errorf("want ErrValidation for whitespace-only email, got %v", err)
	}
}

// Edge: local-only address (no domain part) → ErrValidation.
func TestApprove_LocalOnlyEmail_ReturnsValidationError(t *testing.T) {
	svc := &ProposalService{db: nil}
	_, err := svc.Approve(context.Background(), "any-slug", "user@")
	if err != ErrValidation {
		t.Errorf("want ErrValidation for 'user@', got %v", err)
	}
}

// Happy-path format gate: a well-formed email must NOT return ErrValidation.
// The call will panic at DB access (nil pool); we recover to confirm that the
// email gate was passed and ErrValidation was not returned.
func TestApprove_ValidEmail_PassesValidationGate(t *testing.T) {
	svc := &ProposalService{db: nil}

	gotValidationErr := false
	func() {
		defer func() { recover() }() // suppress nil-pool panic from DB access
		_, err := svc.Approve(context.Background(), "any-slug", "client@example.com")
		if err == ErrValidation {
			gotValidationErr = true
		}
	}()

	if gotValidationErr {
		t.Error("well-formed email must not return ErrValidation")
	}
}

// ─── Integration: Approve (requires TEST_DATABASE_URL) ───────────────────────

// TestApprove_HappyPath_StatusApproved covers AC5: status set to 'approved',
// client_email persisted, and approved_at timestamp set.
func TestApprove_HappyPath_StatusApproved(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	uid := fmt.Sprintf("aqa-approve-%d", time.Now().UnixNano())
	testSlug := fmt.Sprintf("appr%d", time.Now().UnixNano())
	clientEmail := fmt.Sprintf("client-%d@example.com", time.Now().UnixNano())

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Test Agency', 'free') RETURNING id`,
		uid+"@example.com",
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var proposalID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, status, slug, slug_active, blocks)
		 VALUES ($1, 'AQA Approve Test', 'sent', $2, true, '[]') RETURNING id`,
		userID, testSlug,
	).Scan(&proposalID); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM job_queue WHERE payload::text LIKE $1`, "%"+proposalID+"%")
		_, _ = pool.Exec(ctx, `DELETE FROM proposals WHERE id=$1`, proposalID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := NewProposalService(pool)
	approvedAt, err := svc.Approve(ctx, testSlug, clientEmail)
	if err != nil {
		t.Fatalf("Approve error: %v", err)
	}
	if approvedAt == nil {
		t.Fatal("Approve returned nil approvedAt")
	}

	// AC5: verify DB state.
	var status, savedEmail string
	var dbApprovedAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT status, client_email, approved_at FROM proposals WHERE id=$1`,
		proposalID,
	).Scan(&status, &savedEmail, &dbApprovedAt); err != nil {
		t.Fatalf("query proposal: %v", err)
	}

	if status != "approved" {
		t.Errorf("status: want 'approved', got %q", status)
	}
	if savedEmail != clientEmail {
		t.Errorf("client_email: want %q, got %q", clientEmail, savedEmail)
	}
	if dbApprovedAt == nil {
		t.Error("approved_at must be set in DB, got nil")
	}
}

// TestApprove_HappyPath_EnqueuesEmailJobs covers AC6 (email to client) and AC7 (email to owner).
func TestApprove_HappyPath_EnqueuesEmailJobs(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	uid := fmt.Sprintf("aqa-email-%d", time.Now().UnixNano())
	testSlug := fmt.Sprintf("eml%d", time.Now().UnixNano())
	clientEmail := fmt.Sprintf("clieml-%d@example.com", time.Now().UnixNano())
	ownerEmail := uid + "@example.com"

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Test Agency', 'free') RETURNING id`,
		ownerEmail,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var proposalID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, status, slug, slug_active, blocks)
		 VALUES ($1, 'AQA Email Test', 'sent', $2, true, '[]') RETURNING id`,
		userID, testSlug,
	).Scan(&proposalID); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM job_queue WHERE payload::text LIKE $1`, "%"+testSlug+"%")
		_, _ = pool.Exec(ctx, `DELETE FROM proposals WHERE id=$1`, proposalID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := NewProposalService(pool)
	if _, err := svc.Approve(ctx, testSlug, clientEmail); err != nil {
		t.Fatalf("Approve error: %v", err)
	}

	// AC6: email_client_approved job must exist with correct client_email.
	var clientPayload []byte
	if err := pool.QueryRow(ctx,
		`SELECT payload FROM job_queue WHERE job_type='email_client_approved'
		 AND payload->>'client_email'=$1`,
		clientEmail,
	).Scan(&clientPayload); err != nil {
		t.Fatalf("AC6: email_client_approved job not found: %v", err)
	}

	var clientJob map[string]string
	if err := json.Unmarshal(clientPayload, &clientJob); err != nil {
		t.Fatalf("unmarshal client job payload: %v", err)
	}
	if clientJob["client_email"] != clientEmail {
		t.Errorf("AC6: client_email in job: want %q, got %q", clientEmail, clientJob["client_email"])
	}
	if clientJob["agency_name"] == "" {
		t.Error("AC6: agency_name must be set in client job payload")
	}

	// AC7: email_approved_notify job must exist with correct owner_email and client_email.
	var ownerPayload []byte
	if err := pool.QueryRow(ctx,
		`SELECT payload FROM job_queue WHERE job_type='email_approved_notify'
		 AND payload->>'owner_email'=$1`,
		ownerEmail,
	).Scan(&ownerPayload); err != nil {
		t.Fatalf("AC7: email_approved_notify job not found: %v", err)
	}

	var ownerJob map[string]string
	if err := json.Unmarshal(ownerPayload, &ownerJob); err != nil {
		t.Fatalf("unmarshal owner job payload: %v", err)
	}
	if ownerJob["owner_email"] != ownerEmail {
		t.Errorf("AC7: owner_email in job: want %q, got %q", ownerEmail, ownerJob["owner_email"])
	}
	if ownerJob["client_email"] != clientEmail {
		t.Errorf("AC7: client_email in owner job: want %q, got %q", clientEmail, ownerJob["client_email"])
	}
}

// TestApprove_AlreadyApproved_ReturnsConflict covers AC9: second approve → ErrConflict.
func TestApprove_AlreadyApproved_ReturnsConflict(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	uid := fmt.Sprintf("aqa-dup-%d", time.Now().UnixNano())
	testSlug := fmt.Sprintf("dup%d", time.Now().UnixNano())

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Test Agency', 'free') RETURNING id`,
		uid+"@example.com",
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var proposalID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, status, slug, slug_active, blocks)
		 VALUES ($1, 'AQA Dup Test', 'sent', $2, true, '[]') RETURNING id`,
		userID, testSlug,
	).Scan(&proposalID); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM job_queue WHERE payload::text LIKE $1`, "%"+testSlug+"%")
		_, _ = pool.Exec(ctx, `DELETE FROM proposals WHERE id=$1`, proposalID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := NewProposalService(pool)

	// First approval must succeed.
	if _, err := svc.Approve(ctx, testSlug, "first@example.com"); err != nil {
		t.Fatalf("first Approve error: %v", err)
	}

	// Second approval on the same proposal must return ErrConflict.
	_, err := svc.Approve(ctx, testSlug, "second@example.com")
	if err != ErrConflict {
		t.Errorf("AC9: want ErrConflict on second approve, got %v", err)
	}
}
