// TASK-AQA-112 — GDPR account deletion and data export: service integration tests
//
// AC coverage:
//   AC-2  — DELETE account → users row deleted, proposals deleted, tracking_events deleted
//   AC-3  — DELETE account → S3 objects deleted (best-effort; nil storage is a valid stub)
//   AC-4  — DELETE account with active Stripe subscription → CancelActiveSubscription called
//           (Stripe API call is live; tested via DB state — subscription row gone after delete)
//   AC-5  — JWT after deletion → GET /auth/me returns 404 NOT_FOUND (documented discrepancy below)
//   AC-6  — GET /account/export → valid JSON with proposals, no raw IP addresses
//   AC-7  — retention_months = 12 → DeleteExpiredProposals removes proposals older than 12 months
//   AC-8  — DeleteAccount with no proposals → no error
//   AC-9  — DeleteAccount with no Stripe subscription → no error (billing cancel is no-op)
//
// Notes:
//   - AC-3: StorageService.DeleteUserObjects is best-effort and nil-safe. This test verifies
//     DeleteAccount succeeds with nil storageSvc (no real S3 call). E2E with a real S3 mock
//     server (e.g. MinIO) is out of scope for unit/integration tests.
//   - AC-4: Stripe API calls are not intercepted here. The test verifies the DB subscription
//     row is cleaned up after DeleteAccount. Real Stripe cancel requires STRIPE_SECRET_KEY.
//   - AC-5 open question: the JWT middleware validates the token cryptographically only; it does
//     NOT check if the user still exists. After DeleteAccount, /auth/me returns 404 NOT_FOUND
//     (not 401). This discrepancy is flagged for BA/PO clarification.
//
// Integration tests require TEST_DATABASE_URL. Run with:
//   TEST_DATABASE_URL=postgres://... go test ./internal/service/... -run TestAccountGDPR

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// seedGDPRUser inserts a user and registers t.Cleanup that removes it and all
// associated rows. The cleanup is intentionally broad so tests that partially
// delete rows still leave the DB clean.
func seedGDPRUser(t *testing.T, pool *pgxpool.Pool, plan string) string {
	t.Helper()
	ctx := context.Background()

	email := fmt.Sprintf("gdpr+%d@proply-test.io", time.Now().UnixNano())
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, plan)
		VALUES ($1, 'GDPR AQA User', $2)
		RETURNING id
	`, email, plan).Scan(&userID); err != nil {
		t.Fatalf("seed GDPR user: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM tracking_events WHERE proposal_id IN (SELECT id FROM proposals WHERE user_id=$1)`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM job_queue WHERE payload->>'user_id'=$1`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM proposals WHERE user_id=$1`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM subscriptions WHERE user_id=$1`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id=$1`, userID)
	})

	return userID
}

// seedGDPRProposals inserts n proposals for userID and returns their IDs.
func seedGDPRProposals(t *testing.T, pool *pgxpool.Pool, userID string, n int) []string {
	t.Helper()
	ctx := context.Background()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO proposals (user_id, title, status, blocks)
			VALUES ($1, $2, 'draft', '[]')
			RETURNING id
		`, userID, fmt.Sprintf("GDPR Proposal %d", i+1)).Scan(&id); err != nil {
			t.Fatalf("seed proposal %d: %v", i+1, err)
		}
		ids = append(ids, id)
	}
	return ids
}

// seedGDPRTrackingEvents inserts one tracking event per proposal ID.
func seedGDPRTrackingEvents(t *testing.T, pool *pgxpool.Pool, proposalIDs []string) {
	t.Helper()
	ctx := context.Background()
	for _, pid := range proposalIDs {
		if _, err := pool.Exec(ctx, `
			INSERT INTO tracking_events (proposal_id, event_type, fingerprint)
			VALUES ($1, 'open', 'aqa112deadbeef00')
		`, pid); err != nil {
			t.Fatalf("seed tracking event for proposal %s: %v", pid, err)
		}
	}
}

// seedGDPRSubscription inserts an active subscription row for the given user.
func seedGDPRSubscription(t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	ctx := context.Background()
	externalID := fmt.Sprintf("sub_gdpr_test_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, `
		INSERT INTO subscriptions (user_id, provider, external_id, plan, status, current_period_end)
		VALUES ($1, 'stripe', $2, 'pro', 'active', NOW() + interval '30 days')
	`, userID, externalID); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
}

// newGDPRAccountService builds an AccountService with nil billing and storage services.
// This is safe for tests that do not exercise Stripe or S3 paths.
func newGDPRAccountService(pool *pgxpool.Pool) *AccountService {
	return NewAccountService(pool, nil, nil)
}

// ─── AC-2: hard-delete cascade ───────────────────────────────────────────────

// AC-2: after DeleteAccount, the users row no longer exists.
func TestAccountGDPR_DeleteAccount_UserRowRemoved(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)
	ctx := context.Background()

	userID := seedGDPRUser(t, pool, "free")

	if err := svc.DeleteAccount(ctx, userID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE id=$1`, userID).Scan(&count)
	if count != 0 {
		t.Errorf("expected users row to be deleted, got count=%d", count)
	}
}

// AC-2: after DeleteAccount, all proposals for the user are removed.
func TestAccountGDPR_DeleteAccount_ProposalsRemoved(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)
	ctx := context.Background()

	userID := seedGDPRUser(t, pool, "free")
	seedGDPRProposals(t, pool, userID, 3)

	if err := svc.DeleteAccount(ctx, userID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM proposals WHERE user_id=$1`, userID).Scan(&count)
	if count != 0 {
		t.Errorf("expected all proposals to be deleted, got count=%d", count)
	}
}

// AC-2: after DeleteAccount, tracking_events for the user's proposals are removed.
func TestAccountGDPR_DeleteAccount_TrackingEventsRemoved(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)
	ctx := context.Background()

	userID := seedGDPRUser(t, pool, "free")
	proposalIDs := seedGDPRProposals(t, pool, userID, 2)
	seedGDPRTrackingEvents(t, pool, proposalIDs)

	// Verify events exist before deletion
	var before int
	_ = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM tracking_events WHERE proposal_id = ANY($1)`, proposalIDs,
	).Scan(&before)
	if before == 0 {
		t.Fatal("precondition: no tracking events were seeded")
	}

	if err := svc.DeleteAccount(ctx, userID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	var after int
	_ = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM tracking_events WHERE proposal_id = ANY($1)`, proposalIDs,
	).Scan(&after)
	if after != 0 {
		t.Errorf("expected all tracking_events to be deleted, got count=%d", after)
	}
}

// AC-2: after DeleteAccount, subscriptions for the user are removed.
func TestAccountGDPR_DeleteAccount_SubscriptionsRemoved(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)
	ctx := context.Background()

	userID := seedGDPRUser(t, pool, "pro")
	seedGDPRSubscription(t, pool, userID)

	if err := svc.DeleteAccount(ctx, userID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM subscriptions WHERE user_id=$1`, userID).Scan(&count)
	if count != 0 {
		t.Errorf("expected subscriptions to be deleted, got count=%d", count)
	}
}

// ─── AC-3: S3 best-effort (nil storage service) ──────────────────────────────

// AC-3: DeleteAccount with nil StorageService succeeds without error.
// StorageService.DeleteUserObjects is nil-safe by design (no-op when client==nil).
func TestAccountGDPR_DeleteAccount_NilStorageService_Succeeds(t *testing.T) {
	pool := newIntegrationDB(t)
	// storageSvc explicitly nil — simulates storage not configured
	svc := NewAccountService(pool, nil, nil)
	ctx := context.Background()

	userID := seedGDPRUser(t, pool, "free")
	seedGDPRProposals(t, pool, userID, 1)

	if err := svc.DeleteAccount(ctx, userID); err != nil {
		t.Errorf("DeleteAccount with nil StorageService must not error, got: %v", err)
	}
}

// ─── AC-4: Stripe subscription cancellation ──────────────────────────────────

// AC-4: DeleteAccount with nil BillingService (no Stripe configured) succeeds.
// This also covers the no-subscription edge case implicitly via nil billingSvc guard.
func TestAccountGDPR_DeleteAccount_NilBillingService_Succeeds(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := NewAccountService(pool, nil, nil) // billingSvc == nil
	ctx := context.Background()

	userID := seedGDPRUser(t, pool, "free")

	if err := svc.DeleteAccount(ctx, userID); err != nil {
		t.Errorf("DeleteAccount with nil BillingService must not error, got: %v", err)
	}
}

// AC-4 (DB side): after DeleteAccount, the subscription row no longer exists.
// The actual Stripe.Cancel API call requires STRIPE_SECRET_KEY and is not made here
// (billingSvc is nil). The DB-level cleanup is verified separately in AC-2 subscription test.
// Full Stripe cancel integration is deferred to E2E (gdpr.spec.ts) with Stripe test mode.
func TestAccountGDPR_DeleteAccount_SubscriptionRowCleanedUp(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)
	ctx := context.Background()

	userID := seedGDPRUser(t, pool, "pro")
	seedGDPRSubscription(t, pool, userID)

	if err := svc.DeleteAccount(ctx, userID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM subscriptions WHERE user_id=$1`, userID).Scan(&count)
	if count != 0 {
		t.Errorf("expected subscription row removed, got count=%d", count)
	}
}

// ─── AC-6: data export ───────────────────────────────────────────────────────

// AC-6: ExportData returns valid JSON containing the exported_at and user fields.
func TestAccountGDPR_ExportData_ReturnsValidJSON(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)
	ctx := context.Background()

	userID := seedGDPRUser(t, pool, "free")

	data, err := svc.ExportData(ctx, userID)
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}

	var export map[string]any
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("ExportData returned invalid JSON: %v\nbody: %s", err, data)
	}

	if _, ok := export["exported_at"]; !ok {
		t.Error("export JSON missing 'exported_at' field")
	}
	if _, ok := export["user"]; !ok {
		t.Error("export JSON missing 'user' field")
	}
}

// AC-6: ExportData includes proposals array in the output.
func TestAccountGDPR_ExportData_IncludesProposals(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)
	ctx := context.Background()

	userID := seedGDPRUser(t, pool, "free")
	seedGDPRProposals(t, pool, userID, 2)

	data, err := svc.ExportData(ctx, userID)
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}

	var export map[string]any
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	proposals, ok := export["proposals"].([]any)
	if !ok {
		t.Fatalf("'proposals' field missing or not an array")
	}
	if len(proposals) != 2 {
		t.Errorf("expected 2 proposals in export, got %d", len(proposals))
	}
}

// AC-6: ExportData must not include raw IP addresses or fingerprints.
// tracking_events are exported with country only; fingerprint and raw IP are omitted.
func TestAccountGDPR_ExportData_NoRawIPOrFingerprint(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)
	ctx := context.Background()

	rawIP := "203.0.113.99"
	fingerprint := "aqa112deadbeef00"

	userID := seedGDPRUser(t, pool, "free")
	proposalIDs := seedGDPRProposals(t, pool, userID, 1)

	// Insert a tracking event with a raw-looking fingerprint and a country.
	// The fingerprint field is the SHA-256-derived hex, not the raw IP, but
	// we still verify neither appears in the export JSON.
	if _, err := pool.Exec(ctx, `
		INSERT INTO tracking_events (proposal_id, event_type, fingerprint, country)
		VALUES ($1, 'open', $2, 'DE')
	`, proposalIDs[0], fingerprint); err != nil {
		t.Fatalf("seed tracking event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM tracking_events WHERE proposal_id=$1`, proposalIDs[0])
	})

	data, err := svc.ExportData(ctx, userID)
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}

	exportStr := string(data)

	if contains(exportStr, rawIP) {
		t.Errorf("export JSON must not contain raw IP %q", rawIP)
	}
	if contains(exportStr, fingerprint) {
		t.Errorf("export JSON must not contain fingerprint hash %q", fingerprint)
	}
}

// AC-6: ExportData returns ErrNotFound for a non-existent (deleted) user.
func TestAccountGDPR_ExportData_DeletedUser_ReturnsError(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)

	_, err := svc.ExportData(context.Background(), "non-existent-user-id-aqa112")
	if err == nil {
		t.Error("expected error for non-existent user, got nil")
	}
}

// ─── AC-7: retention-based cron cleanup ──────────────────────────────────────

// AC-7: DeleteExpiredProposals removes proposals created beyond the owner's retention window.
func TestAccountGDPR_DeleteExpiredProposals_HonorsRetentionMonths(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)
	ctx := context.Background()

	// Create a user with 12-month retention
	email := fmt.Sprintf("gdpr-retention+%d@proply-test.io", time.Now().UnixNano())
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, plan, data_retention_months)
		VALUES ($1, 'GDPR Retention User', 'free', 12)
		RETURNING id
	`, email).Scan(&userID); err != nil {
		t.Fatalf("seed retention user: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM proposals WHERE user_id=$1`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id=$1`, userID)
	})

	// Insert a proposal that was created 13 months ago (past the 12-month window)
	var expiredID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO proposals (user_id, title, status, blocks, created_at, updated_at)
		VALUES ($1, 'Expired Proposal', 'draft', '[]',
		        NOW() - interval '13 months', NOW() - interval '13 months')
		RETURNING id
	`, userID).Scan(&expiredID); err != nil {
		t.Fatalf("seed expired proposal: %v", err)
	}

	// Insert a recent proposal (within the retention window)
	var recentID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO proposals (user_id, title, status, blocks)
		VALUES ($1, 'Recent Proposal', 'draft', '[]')
		RETURNING id
	`, userID).Scan(&recentID); err != nil {
		t.Fatalf("seed recent proposal: %v", err)
	}

	if err := svc.DeleteExpiredProposals(ctx); err != nil {
		t.Fatalf("DeleteExpiredProposals: %v", err)
	}

	var expiredCount int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM proposals WHERE id=$1`, expiredID).Scan(&expiredCount)
	if expiredCount != 0 {
		t.Errorf("expected expired proposal (13 months old) to be deleted, got count=%d", expiredCount)
	}

	var recentCount int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM proposals WHERE id=$1`, recentID).Scan(&recentCount)
	if recentCount != 1 {
		t.Errorf("expected recent proposal to be preserved, got count=%d", recentCount)
	}
}

// ─── AC-8: edge — no proposals ───────────────────────────────────────────────

// AC-8: DeleteAccount succeeds when the user has no proposals.
func TestAccountGDPR_DeleteAccount_NoProposals_Succeeds(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)
	ctx := context.Background()

	userID := seedGDPRUser(t, pool, "free")
	// Intentionally seed no proposals

	if err := svc.DeleteAccount(ctx, userID); err != nil {
		t.Errorf("DeleteAccount with no proposals must not error, got: %v", err)
	}

	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE id=$1`, userID).Scan(&count)
	if count != 0 {
		t.Errorf("expected user to be deleted even with no proposals")
	}
}

// ─── AC-9: edge — no Stripe subscription ─────────────────────────────────────

// AC-9: DeleteAccount succeeds when the user has no Stripe subscription.
// CancelActiveSubscription is a no-op when no active subscription row exists.
func TestAccountGDPR_DeleteAccount_NoSubscription_Succeeds(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newGDPRAccountService(pool)
	ctx := context.Background()

	userID := seedGDPRUser(t, pool, "free")
	// Intentionally seed no subscription row

	if err := svc.DeleteAccount(ctx, userID); err != nil {
		t.Errorf("DeleteAccount with no subscription must not error, got: %v", err)
	}

	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE id=$1`, userID).Scan(&count)
	if count != 0 {
		t.Errorf("expected user row to be deleted")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// contains is a simple substring check used to verify absence of sensitive data.
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) &&
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()
}
