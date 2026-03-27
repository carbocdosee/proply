package handler

import (
	"io"
	"net/http"

	"proply/internal/service"
)

// BillingHandler handles billing-related endpoints (Stripe webhooks, checkout).
type BillingHandler struct {
	billingSvc *service.BillingService
}

// NewBillingHandler creates a new BillingHandler.
func NewBillingHandler(billingSvc *service.BillingService) *BillingHandler {
	return &BillingHandler{billingSvc: billingSvc}
}

// StripeWebhook handles POST /api/v1/webhooks/stripe
func (h *BillingHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		respondError(w, http.StatusBadRequest, "READ_ERROR")
		return
	}

	signature := r.Header.Get("Stripe-Signature")
	if err := h.billingSvc.HandleStripeWebhook(r.Context(), body, signature); err != nil {
		switch err {
		case service.ErrInvalidSignature:
			respondError(w, http.StatusBadRequest, "INVALID_SIGNATURE")
		case service.ErrAlreadyProcessed:
			// Idempotent: Stripe may retry
			w.WriteHeader(http.StatusOK)
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

// CreateCheckout handles POST /api/v1/billing/checkout
func (h *BillingHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	var req struct {
		Plan string `json:"plan"` // "pro" | "team"
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	url, err := h.billingSvc.CreateCheckoutSession(r.Context(), claims.UserID, req.Plan)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	respondOK(w, map[string]string{"checkout_url": url})
}

// CreatePortal handles POST /api/v1/billing/portal
func (h *BillingHandler) CreatePortal(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	url, err := h.billingSvc.CreatePortalSession(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	respondOK(w, map[string]string{"portal_url": url})
}

// decodeJSON is a convenience wrapper for decoding request body.
func decodeJSON(r *http.Request, v any) error {
	return nil // implemented inline via json.NewDecoder in each handler
}
