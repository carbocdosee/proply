package handler

import (
	"encoding/json"
	"net/http"

	"proply/internal/service"
)

// UploadHandler handles presigned URL generation for direct S3 uploads.
type UploadHandler struct {
	storageSvc *service.StorageService
}

// NewUploadHandler creates a new UploadHandler.
func NewUploadHandler(storageSvc *service.StorageService) *UploadHandler {
	return &UploadHandler{storageSvc: storageSvc}
}

// Presign handles POST /api/v1/upload/presign.
// Returns a short-lived presigned PUT URL and the future public file URL.
func (h *UploadHandler) Presign(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	var req struct {
		FileType    string `json:"file_type"`    // "logo" | "case_study" | "team_member"
		ContentType string `json:"content_type"` // "image/png" | "image/jpeg" | "image/webp"
		SizeBytes   int    `json:"size_bytes"`
		ProposalID  string `json:"proposal_id,omitempty"` // required for case_study / team_member
		BlockID     string `json:"block_id,omitempty"`    // required for case_study / team_member
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}
	if req.FileType == "" || req.ContentType == "" || req.SizeBytes <= 0 {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS")
		return
	}

	result, err := h.storageSvc.PresignUpload(r.Context(), service.PresignUploadInput{
		UserID:      claims.UserID,
		ProposalID:  req.ProposalID,
		BlockID:     req.BlockID,
		FileType:    req.FileType,
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
	})
	if err != nil {
		switch err {
		case service.ErrFileTooLarge:
			respond(w, http.StatusBadRequest, map[string]any{
				"code":      "FILE_TOO_LARGE",
				"max_bytes": 2097152,
			})
		case service.ErrInvalidContentType:
			respondError(w, http.StatusUnprocessableEntity, "INVALID_CONTENT_TYPE")
		case service.ErrStorageNotConfigured:
			respondError(w, http.StatusServiceUnavailable, "STORAGE_NOT_CONFIGURED")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR")
		}
		return
	}

	respondOK(w, result)
}
