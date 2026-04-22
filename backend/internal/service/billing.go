package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v78"
	billingportalsession "github.com/stripe/stripe-go/v78/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v78/checkout/session"
	stripecustomer "github.com/stripe/stripe-go/v78/customer"
	stripesub "github.com/stripe/stripe-go/v78/subscription"
	"github.com/stripe/stripe-go/v78/webhook"

	"proply/internal/config"
)

// BillingService handles Stripe billing operations.
type BillingService struct {
	db  *pgxpool.Pool
	cfg *config.Config
}

// NewBillingService creates a new BillingService.
func NewBillingService(db *pgxpool.Pool, cfg *config.Config) *BillingService {
	stripe.Key = cfg.StripeSecretKey
	return &BillingService{db: db, cfg: cfg}
}

// HandleStripeWebhook processes incoming Stripe webhook events.
func (s *BillingService) HandleStripeWebhook(ctx context.Context, payload []byte, signature string) error {
	event, err := webhook.ConstructEvent(payload, signature, s.cfg.StripeWebhookSecret)
	if err != nil {
		return ErrInvalidSignature
	}

	// Idempotency: skip events already processed
	var alreadyProcessed bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM processed_webhooks WHERE event_id=$1)`, event.ID,
	).Scan(&alreadyProcessed); err == nil && alreadyProcessed {
		return ErrAlreadyProcessed
	}

	switch event.Type {
	case "customer.subscription.created", "customer.subscription.updated":
		if err := s.handleSubscriptionUpsert(ctx, event.Data.Raw); err != nil {
			slog.Error("billing: subscription upsert failed", "event", event.ID, "error", err)
			return err
		}
	case "customer.subscription.deleted":
		if err := s.handleSubscriptionDeleted(ctx, event.Data.Raw); err != nil {
			slog.Error("billing: subscription deleted failed", "event", event.ID, "error", err)
			return err
		}
	case "invoice.payment_failed":
		if err := s.handlePaymentFailed(ctx, event.Data.Raw); err != nil {
			// Non-fatal: log and continue so the event is still marked processed
			slog.Warn("billing: payment_failed handler error", "event", event.ID, "error", err)
		}
	}

	// Record processed event for idempotency
	_, err = s.db.Exec(ctx,
		`INSERT INTO processed_webhooks (event_id, provider) VALUES ($1, 'stripe') ON CONFLICT DO NOTHING`,
		event.ID,
	)
	return err
}

// handleSubscriptionUpsert syncs a created/updated Stripe subscription to the database.
func (s *BillingService) handleSubscriptionUpsert(ctx context.Context, raw json.RawMessage) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(raw, &sub); err != nil {
		return fmt.Errorf("billing: unmarshal subscription: %w", err)
	}

	// Map Stripe price ID → internal plan name
	plan := "free"
	if len(sub.Items.Data) > 0 {
		switch sub.Items.Data[0].Price.ID {
		case s.cfg.StripePriceProID:
			plan = "pro"
		case s.cfg.StripePriceTeamID:
			plan = "team"
		}
	}

	customerID := sub.Customer.ID
	var userID string
	if err := s.db.QueryRow(ctx,
		`SELECT id FROM users WHERE stripe_customer_id=$1 AND deleted_at IS NULL`, customerID,
	).Scan(&userID); err != nil {
		return fmt.Errorf("billing: user not found for customer %s: %w", customerID, err)
	}

	// The active plan shown in the app only reflects active/trialing subscriptions
	activePlan := plan
	if sub.Status != stripe.SubscriptionStatusActive && sub.Status != stripe.SubscriptionStatusTrialing {
		activePlan = "free"
	}

	// Upsert subscription row
	if _, err := s.db.Exec(ctx, `
		INSERT INTO subscriptions (user_id, provider, external_id, plan, status, current_period_end)
		VALUES ($1, 'stripe', $2, $3, $4, TO_TIMESTAMP($5))
		ON CONFLICT (external_id) DO UPDATE SET
			plan               = EXCLUDED.plan,
			status             = EXCLUDED.status,
			current_period_end = EXCLUDED.current_period_end,
			updated_at         = NOW()
	`, userID, sub.ID, plan, string(sub.Status), sub.CurrentPeriodEnd); err != nil {
		return fmt.Errorf("billing: upsert subscription: %w", err)
	}

	_, err := s.db.Exec(ctx, `UPDATE users SET plan=$1 WHERE id=$2`, activePlan, userID)
	return err
}

// handleSubscriptionDeleted downgrades the user to free when a subscription is cancelled.
func (s *BillingService) handleSubscriptionDeleted(ctx context.Context, raw json.RawMessage) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(raw, &sub); err != nil {
		return fmt.Errorf("billing: unmarshal subscription: %w", err)
	}

	customerID := sub.Customer.ID
	var userID string
	if err := s.db.QueryRow(ctx,
		`SELECT id FROM users WHERE stripe_customer_id=$1 AND deleted_at IS NULL`, customerID,
	).Scan(&userID); err != nil {
		return fmt.Errorf("billing: user not found for customer %s: %w", customerID, err)
	}

	if _, err := s.db.Exec(ctx,
		`UPDATE subscriptions SET status='cancelled', updated_at=NOW() WHERE external_id=$1`, sub.ID,
	); err != nil {
		return fmt.Errorf("billing: mark subscription cancelled: %w", err)
	}

	_, err := s.db.Exec(ctx, `UPDATE users SET plan='free' WHERE id=$1`, userID)
	return err
}

// handlePaymentFailed marks the subscription as past_due.
func (s *BillingService) handlePaymentFailed(ctx context.Context, raw json.RawMessage) error {
	var inv stripe.Invoice
	if err := json.Unmarshal(raw, &inv); err != nil {
		return fmt.Errorf("billing: unmarshal invoice: %w", err)
	}
	if inv.Subscription == nil {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`UPDATE subscriptions SET status='past_due', updated_at=NOW() WHERE external_id=$1`,
		inv.Subscription.ID,
	)
	return err
}

// CreateCheckoutSession creates a Stripe Checkout session for plan upgrade.
func (s *BillingService) CreateCheckoutSession(ctx context.Context, userID, plan string) (string, error) {
	priceID := s.cfg.StripePriceProID
	if plan == "team" {
		priceID = s.cfg.StripePriceTeamID
	}
	if priceID == "" {
		return "", fmt.Errorf("billing: price ID not configured for plan %s", plan)
	}

	customerID, err := s.ensureStripeCustomer(ctx, userID)
	if err != nil {
		return "", err
	}

	sess, err := checkoutsession.New(&stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(priceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL: stripe.String(s.cfg.AppURL + "/dashboard/billing?success=1"),
		CancelURL:  stripe.String(s.cfg.AppURL + "/dashboard/billing"),
	})
	if err != nil {
		return "", fmt.Errorf("billing: create checkout session: %w", err)
	}
	return sess.URL, nil
}

// CreatePortalSession creates a Stripe Customer Portal session for subscription management.
func (s *BillingService) CreatePortalSession(ctx context.Context, userID string) (string, error) {
	customerID, err := s.ensureStripeCustomer(ctx, userID)
	if err != nil {
		return "", err
	}

	sess, err := billingportalsession.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(s.cfg.AppURL + "/dashboard/billing"),
	})
	if err != nil {
		return "", fmt.Errorf("billing: create portal session: %w", err)
	}
	return sess.URL, nil
}

// ensureStripeCustomer returns the Stripe customer ID for a user,
// creating a new Stripe Customer if one does not yet exist.
func (s *BillingService) ensureStripeCustomer(ctx context.Context, userID string) (string, error) {
	var email, name string
	var customerID *string
	if err := s.db.QueryRow(ctx,
		`SELECT email, name, stripe_customer_id FROM users WHERE id=$1 AND deleted_at IS NULL`, userID,
	).Scan(&email, &name, &customerID); err != nil {
		return "", fmt.Errorf("billing: lookup user: %w", err)
	}

	if customerID != nil && *customerID != "" {
		return *customerID, nil
	}

	// Create a new Stripe Customer and persist the ID
	cust, err := stripecustomer.New(&stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
		Metadata: map[string]string{"proply_user_id": userID},
	})
	if err != nil {
		return "", fmt.Errorf("billing: create stripe customer: %w", err)
	}

	if _, err := s.db.Exec(ctx,
		`UPDATE users SET stripe_customer_id=$1 WHERE id=$2`, cust.ID, userID,
	); err != nil {
		return "", fmt.Errorf("billing: save stripe_customer_id: %w", err)
	}
	return cust.ID, nil
}

// CancelActiveSubscription cancels the user's active Stripe subscription immediately.
// No-op if the user has no active subscription or no Stripe customer.
func (s *BillingService) CancelActiveSubscription(ctx context.Context, userID string) error {
	// Look up the active subscription external ID
	var externalID string
	err := s.db.QueryRow(ctx, `
		SELECT external_id FROM subscriptions
		WHERE user_id = $1 AND status IN ('active', 'trialing')
		ORDER BY created_at DESC LIMIT 1
	`, userID).Scan(&externalID)
	if err != nil {
		return nil // no active subscription — nothing to cancel
	}

	_, err = stripesub.Cancel(externalID, nil)
	if err != nil {
		return fmt.Errorf("billing: cancel subscription %s: %w", externalID, err)
	}
	return nil
}
