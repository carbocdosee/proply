package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"proply/internal/service"
)

// PublicHandler handles unauthenticated endpoints for public proposal viewing.
type PublicHandler struct {
	proposalSvc *service.ProposalService
}

// NewPublicHandler creates a new PublicHandler.
func NewPublicHandler(proposalSvc *service.ProposalService) *PublicHandler {
	return &PublicHandler{proposalSvc: proposalSvc}
}

// GetProposal handles GET /api/v1/public/proposals/:slug
// Returns proposal data for the public viewer (SSR from SvelteKit).
func (h *PublicHandler) GetProposal(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	proposal, err := h.proposalSvc.GetPublic(r.Context(), slug)
	if err != nil {
		switch err {
		case service.ErrNotFound:
			respondError(w, http.StatusNotFound, "NOT_FOUND")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	// If password protected and no password provided, return 401 with flag
	if proposal.PasswordProtected {
		password := r.Header.Get("X-Proposal-Password")
		if password == "" {
			respond(w, http.StatusUnauthorized, map[string]any{
				"code":               "PASSWORD_REQUIRED",
				"password_protected": true,
			})
			return
		}
		// Verify password via service
		if err := h.proposalSvc.VerifyProposalPassword(r.Context(), slug, password); err != nil {
			respondError(w, http.StatusUnauthorized, "INVALID_PASSWORD")
			return
		}
	}

	respondOK(w, proposal)
}

// Approve handles POST /api/v1/public/proposals/:slug/approve
func (h *PublicHandler) Approve(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var req struct {
		ClientEmail string `json:"client_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	approvedAt, err := h.proposalSvc.Approve(r.Context(), slug, req.ClientEmail)
	if err != nil {
		switch err {
		case service.ErrNotFound:
			respondError(w, http.StatusNotFound, "NOT_FOUND")
		case service.ErrConflict:
			respondError(w, http.StatusConflict, "ALREADY_APPROVED")
		case service.ErrValidation:
			respondError(w, http.StatusUnprocessableEntity, "INVALID_EMAIL")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respondOK(w, map[string]string{"approved_at": approvedAt.Format("2006-01-02T15:04:05Z07:00")})
}
