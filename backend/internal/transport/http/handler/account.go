package handler

import (
	"encoding/json"
	"net/http"

	"proply/internal/service"
)

// UpdateRetention handles PATCH /api/v1/account/retention
// Sets data_retention_months to 12, 24, or 36.
func (h *AccountHandler) UpdateRetention(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	var body struct {
		Months int `json:"months"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	if err := h.accountSvc.UpdateRetention(r.Context(), claims.UserID, body.Months); err != nil {
		switch err {
		case service.ErrValidation:
			respondError(w, http.StatusUnprocessableEntity, "INVALID_RETENTION_MONTHS")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respondOK(w, map[string]string{"status": "ok"})
}

// ExportData handles GET /api/v1/account/export
// Returns a JSON file with all user data per GDPR right to data portability.
func (h *AccountHandler) ExportData(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	data, err := h.accountSvc.ExportData(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"proply-export.json\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// DeleteAccount handles DELETE /api/v1/account
// Permanently deletes the authenticated user and all their data (GDPR hard delete).
func (h *AccountHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	if err := h.accountSvc.DeleteAccount(r.Context(), claims.UserID); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AccountHandler handles user profile and branding settings.
type AccountHandler struct {
	accountSvc *service.AccountService
}

// NewAccountHandler creates a new AccountHandler.
func NewAccountHandler(accountSvc *service.AccountService) *AccountHandler {
	return &AccountHandler{accountSvc: accountSvc}
}

// UpdateProfile handles PATCH /api/v1/account
// Updates name and/or language. Only provided (non-null) fields are changed.
func (h *AccountHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	var req service.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	if err := h.accountSvc.UpdateProfile(r.Context(), claims.UserID, req); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	respondOK(w, map[string]string{"status": "ok"})
}

// UpdateBranding handles PATCH /api/v1/account/branding
// Updates logo_url, primary_color, accent_color, hide_proply_footer.
// Returns 402 if the user tries to hide the footer on a free plan.
func (h *AccountHandler) UpdateBranding(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	var req service.UpdateBrandingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	if err := h.accountSvc.UpdateBranding(r.Context(), claims.UserID, req); err != nil {
		switch err {
		case service.ErrValidation:
			respondError(w, http.StatusUnprocessableEntity, "INVALID_COLOR")
		case service.ErrPlanRequired:
			respond(w, http.StatusPaymentRequired, map[string]any{
				"code":     "PLAN_REQUIRED",
				"feature":  "hide_proply_footer",
				"min_plan": "pro",
			})
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respond(w, http.StatusOK, map[string]any{})
}
