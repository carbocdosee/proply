package handler

import (
	"encoding/json"
	"net/http"

	"proply/internal/service"
)

// TrackingHandler handles public tracking endpoints (called server-to-server from SvelteKit).
type TrackingHandler struct {
	trackingSvc *service.TrackingService
}

// NewTrackingHandler creates a new TrackingHandler.
func NewTrackingHandler(trackingSvc *service.TrackingService) *TrackingHandler {
	return &TrackingHandler{trackingSvc: trackingSvc}
}

// TrackOpen handles POST /api/v1/internal/track/open
// Called server-to-server from SvelteKit +page.server.ts on every /p/{slug} render.
func (h *TrackingHandler) TrackOpen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug      string `json:"slug"`
		UserAgent string `json:"user_agent"`
		IP        string `json:"ip"`
		CFCountry string `json:"cf_country"` // Cloudflare CF-IPCountry header
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	if err := h.trackingSvc.TrackOpen(r.Context(), req.Slug, req.IP, req.UserAgent, req.CFCountry); err != nil {
		// Non-critical: log but don't block proposal render
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// TrackBlocks handles POST /api/v1/track/block-time
// Called from client-side Intersection Observer (first-party request).
func (h *TrackingHandler) TrackBlocks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug   string `json:"slug"`
		Events []struct {
			BlockID    string `json:"block_id"`
			DurationMs int    `json:"duration_ms"`
		} `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	// Non-blocking: fire and ignore errors (best-effort tracking)
	go func() {
		for _, evt := range req.Events {
			_ = h.trackingSvc.TrackBlockTime(r.Context(), req.Slug, evt.BlockID, evt.DurationMs)
		}
	}()

	w.WriteHeader(http.StatusOK)
}
