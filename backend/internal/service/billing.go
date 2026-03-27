package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v78"
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

	// Idempotency: check if this event was already processed
	var alreadyProcessed bool
	err = s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM processed_webhooks WHERE event_id=$1)`, event.ID,
	).Scan(&alreadyProcessed)
	if err == nil && alreadyProcessed {
		return ErrAlreadyProcessed
	}

	switch event.Type {
	case "customer.subscription.created", "customer.subscription.updated":
		// TODO: extract subscription data and update users.plan + subscriptions table
	case "customer.subscription.deleted":
		// TODO: downgrade user to free plan
	case "invoice.payment_failed":
		// TODO: update subscription status to past_due, enqueue email
	}

	// Mark event as processed (idempotency)
	_, err = s.db.Exec(ctx,
		`INSERT INTO processed_webhooks (event_id, provider) VALUES ($1, 'stripe') ON CONFLICT DO NOTHING`,
		event.ID,
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

	// TODO: implement Stripe Checkout session creation
	// params := &stripe.CheckoutSessionParams{...}
	// session, err := session.New(params)
	return s.cfg.AppURL + "/billing?plan=" + plan, nil
}

// CreatePortalSession creates a Stripe Customer Portal session.
func (s *BillingService) CreatePortalSession(ctx context.Context, userID string) (string, error) {
	// TODO: look up stripe_customer_id for user, create portal session
	return s.cfg.AppURL + "/billing/portal", nil
}
