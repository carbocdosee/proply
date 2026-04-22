// TASK-AQA-111 — Billing (Stripe) service integration tests
//
// AC coverage:
//   AC-2  — webhook subscription.created → users.plan = 'pro'
//   AC-3  — webhook subscription.deleted → users.plan = 'free'
//   AC-4  — duplicate event_id → ErrAlreadyProcessed (no duplicate data)
//   AC-5  — invalid Stripe signature → ErrInvalidSignature (unit, no DB)
//   AC-8  — Free user after cancellation → 4th proposal → ErrPlanLimit
//
// Notes:
//   - AC-1 (checkout session creation) is tested at the handler layer (billing_handler_test.go)
//     and at E2E level (e2e/billing.spec.ts). The service call hits real Stripe SDK
//     and cannot be unit-tested without a network mock.
//   - Integration tests require TEST_DATABASE_URL. Run with:
//       TEST_DATABASE_URL=postgres://... go test ./internal/service/... -run TestBilling
//
// Open questions:
//   OQ-1: AC-5 states the webhook should return 401, but the handler returns 400 for invalid
//         signatures (matching the Stripe recommendation). Flagged for BA/PO clarification.

package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v78"
	"proply/internal/config"
)

// ─── Test constants ───────────────────────────────────────────────────────────

const (
	testBillingWebhookSecret = "whsec_aqa111_test_secret_do_not_use_in_prod"
	testBillingPriceProID    = "price_pro_aqa111_test"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// signStripeWebhook returns a valid Stripe-Signature header value for payload.
// Uses HMAC-SHA256 of "<timestamp>.<payload>" as required by the Stripe webhook spec.
func signStripeWebhook(payload []byte, secret string) string {
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.", ts)
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", ts, sig)
}

// newTestBillingService returns a BillingService with test config.
// The Stripe API key is set to a placeholder; webhook tests do not make real API calls.
func newTestBillingService(pool *pgxpool.Pool) *BillingService {
	cfg := &config.Config{
		StripeSecretKey:     "sk_test_placeholder_aqa111",
		StripeWebhookSecret: testBillingWebhookSecret,
		StripePriceProID:    testBillingPriceProID,
	}
	return NewBillingService(pool, cfg)
}

// stripeEventJSON builds a minimal Stripe webhook event JSON for subscription events.
// The api_version field must match stripe.APIVersion to pass the SDK's version check.
func stripeEventJSON(eventID, eventType, customerID, subID, status, priceID string) []byte {
	return []byte(fmt.Sprintf(`{
		"id": %q,
		"object": "event",
		"api_version": %q,
		"type": %q,
		"data": {
			"object": {
				"id": %q,
				"object": "subscription",
				"customer": %q,
				"status": %q,
				"current_period_end": 9999999999,
				"items": {
					"object": "list",
					"data": [
						{
							"id": "si_test_aqa111",
							"object": "subscription_item",
							"price": { "id": %q }
						}
					]
				}
			}
		}
	}`, eventID, stripe.APIVersion, eventType, subID, customerID, status, priceID))
}

// stripeDeletedEventJSON builds a minimal subscription.deleted event (no items required).
func stripeDeletedEventJSON(eventID, customerID, subID string) []byte {
	return []byte(fmt.Sprintf(`{
		"id": %q,
		"object": "event",
		"api_version": %q,
		"type": "customer.subscription.deleted",
		"data": {
			"object": {
				"id": %q,
				"object": "subscription",
				"customer": %q,
				"status": "canceled",
				"current_period_end": 9999999999,
				"items": {
					"object": "list",
					"data": []
				}
			}
		}
	}`, eventID, stripe.APIVersion, subID, customerID))
}

// seedBillingUser inserts a user with the given stripe_customer_id and plan.
// Registers cleanup that removes the user and related rows.
func seedBillingUser(t *testing.T, pool *pgxpool.Pool, customerID, plan string) string {
	t.Helper()
	ctx := context.Background()

	email := fmt.Sprintf("billing+%d@proply-test.io", time.Now().UnixNano())
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, plan, stripe_customer_id)
		VALUES ($1, 'Billing AQA User', $2, $3)
		RETURNING id
	`, email, plan, customerID).Scan(&userID); err != nil {
		t.Fatalf("seed billing user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM subscriptions WHERE user_id=$1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM proposals WHERE user_id=$1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM processed_webhooks WHERE event_id LIKE 'evt_test_aqa111%'`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	})

	return userID
}

// seedSubscription inserts a subscription row and returns the row's internal ID.
func seedSubscription(t *testing.T, pool *pgxpool.Pool, userID, externalID, plan, status string) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO subscriptions (user_id, provider, external_id, plan, status, current_period_end)
		VALUES ($1, 'stripe', $2, $3, $4, NOW() + interval '30 days')
	`, userID, externalID, plan, status); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
}

// seedDraftProposals inserts n draft proposals for userID and registers cleanup.
func seedDraftProposals(t *testing.T, pool *pgxpool.Pool, userID string, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		if _, err := pool.Exec(ctx, `
			INSERT INTO proposals (user_id, title, status, blocks)
			VALUES ($1, $2, 'draft', '[]')
		`, userID, fmt.Sprintf("AQA Proposal %d", i+1)); err != nil {
			t.Fatalf("seed draft proposal %d: %v", i+1, err)
		}
	}
}

// ─── Unit: signature validation (no DB required) ─────────────────────────────

// AC-5: invalid Stripe signature → ErrInvalidSignature
func TestBillingWebhook_InvalidSignature_ReturnsErrInvalidSignature(t *testing.T) {
	// nil DB pool is safe here: signature check fires before any DB access
	svc := newTestBillingService(nil)
	ctx := context.Background()

	payload := []byte(`{"id":"evt_bad","type":"customer.subscription.created","data":{"object":{}}}`)
	badSig := "t=1234567890,v1=badbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbad"

	err := svc.HandleStripeWebhook(ctx, payload, badSig)
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

// AC-5: empty signature → ErrInvalidSignature
func TestBillingWebhook_EmptySignature_ReturnsErrInvalidSignature(t *testing.T) {
	svc := newTestBillingService(nil)
	payload := []byte(`{"id":"evt_empty_sig","type":"ping","data":{"object":{}}}`)

	err := svc.HandleStripeWebhook(context.Background(), payload, "")
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature for empty signature, got %v", err)
	}
}

// ─── Integration: webhook processing (requires TEST_DATABASE_URL) ────────────

// AC-2: subscription.created with pro price → users.plan set to 'pro'
func TestBillingWebhook_SubscriptionCreated_SetsPlanPro(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newTestBillingService(pool)
	ctx := context.Background()

	customerID := fmt.Sprintf("cus_test_aqa111_ac2_%d", time.Now().UnixNano())
	userID := seedBillingUser(t, pool, customerID, "free")

	payload := stripeEventJSON(
		"evt_test_aqa111_ac2",
		"customer.subscription.created",
		customerID,
		"sub_test_aqa111_ac2",
		"active",
		testBillingPriceProID,
	)
	sig := signStripeWebhook(payload, testBillingWebhookSecret)

	if err := svc.HandleStripeWebhook(ctx, payload, sig); err != nil {
		t.Fatalf("HandleStripeWebhook: %v", err)
	}

	var plan string
	if err := pool.QueryRow(ctx, `SELECT plan FROM users WHERE id=$1`, userID).Scan(&plan); err != nil {
		t.Fatalf("query plan: %v", err)
	}
	if plan != "pro" {
		t.Errorf("expected plan=pro after subscription.created, got %q", plan)
	}
}

// AC-2: subscription.created inserts a row into subscriptions table
func TestBillingWebhook_SubscriptionCreated_InsertsSubscriptionRow(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newTestBillingService(pool)
	ctx := context.Background()

	customerID := fmt.Sprintf("cus_test_aqa111_sub_row_%d", time.Now().UnixNano())
	subID := fmt.Sprintf("sub_test_aqa111_row_%d", time.Now().UnixNano())
	userID := seedBillingUser(t, pool, customerID, "free")

	payload := stripeEventJSON(
		fmt.Sprintf("evt_test_aqa111_row_%d", time.Now().UnixNano()),
		"customer.subscription.created",
		customerID,
		subID,
		"active",
		testBillingPriceProID,
	)
	sig := signStripeWebhook(payload, testBillingWebhookSecret)

	if err := svc.HandleStripeWebhook(ctx, payload, sig); err != nil {
		t.Fatalf("HandleStripeWebhook: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM subscriptions
		WHERE user_id=$1 AND external_id=$2 AND plan='pro' AND status='active'
	`, userID, subID).Scan(&count); err != nil {
		t.Fatalf("query subscriptions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 subscription row, got %d", count)
	}
}

// AC-2: subscription with non-active status → users.plan = 'free' (no promotion)
func TestBillingWebhook_SubscriptionCreated_PastDueStatus_PlanRemainsOrSetsFree(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newTestBillingService(pool)
	ctx := context.Background()

	customerID := fmt.Sprintf("cus_test_aqa111_pastdue_%d", time.Now().UnixNano())
	userID := seedBillingUser(t, pool, customerID, "free")

	payload := stripeEventJSON(
		fmt.Sprintf("evt_test_aqa111_pastdue_%d", time.Now().UnixNano()),
		"customer.subscription.updated",
		customerID,
		fmt.Sprintf("sub_test_aqa111_pastdue_%d", time.Now().UnixNano()),
		"past_due", // not active or trialing
		testBillingPriceProID,
	)
	sig := signStripeWebhook(payload, testBillingWebhookSecret)

	if err := svc.HandleStripeWebhook(ctx, payload, sig); err != nil {
		t.Fatalf("HandleStripeWebhook: %v", err)
	}

	var plan string
	if err := pool.QueryRow(ctx, `SELECT plan FROM users WHERE id=$1`, userID).Scan(&plan); err != nil {
		t.Fatalf("query plan: %v", err)
	}
	// past_due subscription → activePlan resolved to "free"
	if plan != "free" {
		t.Errorf("expected plan=free for past_due subscription, got %q", plan)
	}
}

// AC-3: subscription.deleted → users.plan = 'free'
func TestBillingWebhook_SubscriptionDeleted_SetsPlanFree(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newTestBillingService(pool)
	ctx := context.Background()

	customerID := fmt.Sprintf("cus_test_aqa111_ac3_%d", time.Now().UnixNano())
	subID := fmt.Sprintf("sub_test_aqa111_ac3_%d", time.Now().UnixNano())
	userID := seedBillingUser(t, pool, customerID, "pro")
	seedSubscription(t, pool, userID, subID, "pro", "active")

	payload := stripeDeletedEventJSON(
		fmt.Sprintf("evt_test_aqa111_ac3_%d", time.Now().UnixNano()),
		customerID,
		subID,
	)
	sig := signStripeWebhook(payload, testBillingWebhookSecret)

	if err := svc.HandleStripeWebhook(ctx, payload, sig); err != nil {
		t.Fatalf("HandleStripeWebhook subscription.deleted: %v", err)
	}

	var plan string
	if err := pool.QueryRow(ctx, `SELECT plan FROM users WHERE id=$1`, userID).Scan(&plan); err != nil {
		t.Fatalf("query plan: %v", err)
	}
	if plan != "free" {
		t.Errorf("expected plan=free after subscription.deleted, got %q", plan)
	}
}

// AC-3: subscription.deleted → subscriptions row status = 'cancelled'
func TestBillingWebhook_SubscriptionDeleted_SubscriptionStatusCancelled(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newTestBillingService(pool)
	ctx := context.Background()

	customerID := fmt.Sprintf("cus_test_aqa111_cancel_%d", time.Now().UnixNano())
	subID := fmt.Sprintf("sub_test_aqa111_cancel_%d", time.Now().UnixNano())
	userID := seedBillingUser(t, pool, customerID, "pro")
	seedSubscription(t, pool, userID, subID, "pro", "active")

	payload := stripeDeletedEventJSON(
		fmt.Sprintf("evt_test_aqa111_cancel_%d", time.Now().UnixNano()),
		customerID,
		subID,
	)
	sig := signStripeWebhook(payload, testBillingWebhookSecret)

	if err := svc.HandleStripeWebhook(ctx, payload, sig); err != nil {
		t.Fatalf("HandleStripeWebhook: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx, `
		SELECT status FROM subscriptions WHERE external_id=$1
	`, subID).Scan(&status); err != nil {
		t.Fatalf("query subscription status: %v", err)
	}
	if status != "cancelled" {
		t.Errorf("expected subscription status=cancelled, got %q", status)
	}
}

// AC-4: same event_id sent twice → second call returns ErrAlreadyProcessed
func TestBillingWebhook_DuplicateEventID_ReturnsAlreadyProcessed(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newTestBillingService(pool)
	ctx := context.Background()

	customerID := fmt.Sprintf("cus_test_aqa111_idem_%d", time.Now().UnixNano())
	eventID := fmt.Sprintf("evt_test_aqa111_idem_%d", time.Now().UnixNano())
	seedBillingUser(t, pool, customerID, "free")

	payload := stripeEventJSON(
		eventID,
		"customer.subscription.created",
		customerID,
		fmt.Sprintf("sub_test_aqa111_idem_%d", time.Now().UnixNano()),
		"active",
		testBillingPriceProID,
	)

	// First call — must succeed
	sig1 := signStripeWebhook(payload, testBillingWebhookSecret)
	if err := svc.HandleStripeWebhook(ctx, payload, sig1); err != nil {
		t.Fatalf("first webhook call: %v", err)
	}

	// Second call with the same event_id — must return ErrAlreadyProcessed
	sig2 := signStripeWebhook(payload, testBillingWebhookSecret)
	err := svc.HandleStripeWebhook(ctx, payload, sig2)
	if err != ErrAlreadyProcessed {
		t.Errorf("expected ErrAlreadyProcessed on duplicate event, got %v", err)
	}
}

// AC-4: duplicate event does not create extra subscription rows
func TestBillingWebhook_DuplicateEventID_NoExtraSubscriptionRows(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newTestBillingService(pool)
	ctx := context.Background()

	customerID := fmt.Sprintf("cus_test_aqa111_nodup_%d", time.Now().UnixNano())
	subID := fmt.Sprintf("sub_test_aqa111_nodup_%d", time.Now().UnixNano())
	eventID := fmt.Sprintf("evt_test_aqa111_nodup_%d", time.Now().UnixNano())
	userID := seedBillingUser(t, pool, customerID, "free")

	payload := stripeEventJSON(eventID, "customer.subscription.created", customerID, subID, "active", testBillingPriceProID)

	// Send event twice
	_ = svc.HandleStripeWebhook(ctx, payload, signStripeWebhook(payload, testBillingWebhookSecret))
	_ = svc.HandleStripeWebhook(ctx, payload, signStripeWebhook(payload, testBillingWebhookSecret))

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM subscriptions WHERE user_id=$1 AND external_id=$2`,
		userID, subID,
	).Scan(&count); err != nil {
		t.Fatalf("query subscriptions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 subscription row after duplicate event, got %d", count)
	}
}

// AC-4: duplicate event does not create extra processed_webhooks rows
func TestBillingWebhook_DuplicateEventID_OneProcessedWebhookRow(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newTestBillingService(pool)
	ctx := context.Background()

	customerID := fmt.Sprintf("cus_test_aqa111_pwrow_%d", time.Now().UnixNano())
	eventID := fmt.Sprintf("evt_test_aqa111_pwrow_%d", time.Now().UnixNano())
	seedBillingUser(t, pool, customerID, "free")

	payload := stripeEventJSON(eventID, "customer.subscription.created", customerID,
		fmt.Sprintf("sub_test_aqa111_pwrow_%d", time.Now().UnixNano()), "active", testBillingPriceProID)

	_ = svc.HandleStripeWebhook(ctx, payload, signStripeWebhook(payload, testBillingWebhookSecret))
	_ = svc.HandleStripeWebhook(ctx, payload, signStripeWebhook(payload, testBillingWebhookSecret))

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM processed_webhooks WHERE event_id=$1`, eventID,
	).Scan(&count); err != nil {
		t.Fatalf("query processed_webhooks: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 processed_webhooks row, got %d", count)
	}
}

// AC-8: Free user (after subscription.deleted) → 4th proposal → ErrPlanLimit
func TestBillingWebhook_FreeAfterCancellation_FourthProposalBlocked(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := newTestBillingService(pool)
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	customerID := fmt.Sprintf("cus_test_aqa111_ac8_%d", time.Now().UnixNano())
	subID := fmt.Sprintf("sub_test_aqa111_ac8_%d", time.Now().UnixNano())
	eventID := fmt.Sprintf("evt_test_aqa111_ac8_%d", time.Now().UnixNano())
	userID := seedBillingUser(t, pool, customerID, "pro")
	seedSubscription(t, pool, userID, subID, "pro", "active")

	// Seed 3 existing proposals (at plan limit for free)
	seedDraftProposals(t, pool, userID, 3)

	// Downgrade via webhook
	payload := stripeDeletedEventJSON(eventID, customerID, subID)
	sig := signStripeWebhook(payload, testBillingWebhookSecret)
	if err := svc.HandleStripeWebhook(ctx, payload, sig); err != nil {
		t.Fatalf("subscription.deleted webhook: %v", err)
	}

	// Verify plan is now free
	var plan string
	if err := pool.QueryRow(ctx, `SELECT plan FROM users WHERE id=$1`, userID).Scan(&plan); err != nil {
		t.Fatalf("query plan: %v", err)
	}
	if plan != "free" {
		t.Errorf("expected plan=free after deletion, got %q", plan)
	}

	// Attempt to create 4th proposal — must be blocked
	_, err := proposalSvc.Create(ctx, userID, "free", "Fourth Proposal", "", nil)
	if err != ErrPlanLimit {
		t.Errorf("expected ErrPlanLimit for 4th proposal on free plan, got %v", err)
	}
}
