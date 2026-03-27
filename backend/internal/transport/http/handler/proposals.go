package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"proply/internal/domain"
	"proply/internal/service"
)

// ProposalHandler handles proposal CRUD and lifecycle endpoints.
type ProposalHandler struct {
	proposalSvc *service.ProposalService
}

// NewProposalHandler creates a new ProposalHandler.
func NewProposalHandler(proposalSvc *service.ProposalService) *ProposalHandler {
	return &ProposalHandler{proposalSvc: proposalSvc}
}

// List handles GET /api/v1/proposals
func (h *ProposalHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	filter := service.ProposalFilter{
		UserID:  claims.UserID,
		Status:  r.URL.Query().Get("status"),
		Search:  r.URL.Query().Get("search"),
		Sort:    r.URL.Query().Get("sort"),
		Order:   r.URL.Query().Get("order"),
		Page:    parseIntParam(r.URL.Query().Get("page"), 1),
		PerPage: parseIntParam(r.URL.Query().Get("per_page"), 20),
	}
	if filter.PerPage > 100 {
		filter.PerPage = 100
	}

	result, err := h.proposalSvc.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	respondOK(w, result)
}

// Create handles POST /api/v1/proposals
func (h *ProposalHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	var req struct {
		Title      string  `json:"title"`
		ClientName string  `json:"client_name"`
		TemplateID *string `json:"template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	proposal, err := h.proposalSvc.Create(r.Context(), claims.UserID, claims.Plan, req.Title, req.ClientName, req.TemplateID)
	if err != nil {
		switch err {
		case service.ErrPlanLimit:
			respond(w, http.StatusPaymentRequired, map[string]any{"code": "PLAN_LIMIT", "limit": 3})
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respond(w, http.StatusCreated, map[string]string{"id": proposal.ID})
}

// Get handles GET /api/v1/proposals/:id
func (h *ProposalHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	id := chi.URLParam(r, "id")
	proposal, err := h.proposalSvc.GetByID(r.Context(), id, claims.UserID)
	if err != nil {
		switch err {
		case service.ErrNotFound:
			respondError(w, http.StatusNotFound, "NOT_FOUND")
		case service.ErrForbidden:
			respondError(w, http.StatusForbidden, "FORBIDDEN")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respondOK(w, proposal)
}

// Update handles PATCH /api/v1/proposals/:id
func (h *ProposalHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	id := chi.URLParam(r, "id")

	var req struct {
		Title      *string         `json:"title"`
		ClientName *string         `json:"client_name"`
		Blocks     []domain.Block  `json:"blocks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	updatedAt, err := h.proposalSvc.Update(r.Context(), id, claims.UserID, req.Title, req.ClientName, req.Blocks)
	if err != nil {
		switch err {
		case service.ErrNotFound:
			respondError(w, http.StatusNotFound, "NOT_FOUND")
		case service.ErrForbidden:
			respondError(w, http.StatusForbidden, "FORBIDDEN")
		case service.ErrConflict:
			respondError(w, http.StatusConflict, "APPROVED_READ_ONLY")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respondOK(w, map[string]string{"updated_at": updatedAt.Format("2006-01-02T15:04:05Z07:00")})
}

// Publish handles POST /api/v1/proposals/:id/publish
func (h *ProposalHandler) Publish(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	if !claims.EmailVerified {
		respondError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED")
		return
	}

	id := chi.URLParam(r, "id")
	slug, err := h.proposalSvc.Publish(r.Context(), id, claims.UserID, claims.Plan)
	if err != nil {
		switch err {
		case service.ErrPlanLimit:
			respond(w, http.StatusPaymentRequired, map[string]any{"code": "PLAN_LIMIT", "limit": 3})
		case service.ErrConflict:
			respondError(w, http.StatusConflict, "ALREADY_PUBLISHED")
		case service.ErrNotFound:
			respondError(w, http.StatusNotFound, "NOT_FOUND")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respondOK(w, map[string]string{"slug": slug})
}

// UpdateStatus handles PATCH /api/v1/proposals/:id/status
func (h *ProposalHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	id := chi.URLParam(r, "id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	status, err := h.proposalSvc.UpdateStatus(r.Context(), id, claims.UserID, req.Status)
	if err != nil {
		switch err {
		case service.ErrConflict:
			respondError(w, http.StatusConflict, "INVALID_TRANSITION")
		case service.ErrNotFound:
			respondError(w, http.StatusNotFound, "NOT_FOUND")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respondOK(w, map[string]string{"status": status})
}

// Revoke handles POST /api/v1/proposals/:id/revoke
func (h *ProposalHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.proposalSvc.Revoke(r.Context(), id, claims.UserID); err != nil {
		switch err {
		case service.ErrNotFound:
			respondError(w, http.StatusNotFound, "NOT_FOUND")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respondOK(w, map[string]bool{"slug_active": false})
}

// Duplicate handles POST /api/v1/proposals/:id/duplicate
func (h *ProposalHandler) Duplicate(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	id := chi.URLParam(r, "id")
	newID, err := h.proposalSvc.Duplicate(r.Context(), id, claims.UserID)
	if err != nil {
		switch err {
		case service.ErrNotFound:
			respondError(w, http.StatusNotFound, "NOT_FOUND")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respond(w, http.StatusCreated, map[string]string{"id": newID})
}

// Delete handles DELETE /api/v1/proposals/:id
func (h *ProposalHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.proposalSvc.Delete(r.Context(), id, claims.UserID); err != nil {
		switch err {
		case service.ErrNotFound:
			respondError(w, http.StatusNotFound, "NOT_FOUND")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetAnalytics handles GET /api/v1/proposals/:id/analytics
func (h *ProposalHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	id := chi.URLParam(r, "id")
	analytics, err := h.proposalSvc.GetAnalytics(r.Context(), id, claims.UserID, claims.Plan)
	if err != nil {
		switch err {
		case service.ErrNotFound:
			respondError(w, http.StatusNotFound, "NOT_FOUND")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respondOK(w, analytics)
}

func parseIntParam(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}
