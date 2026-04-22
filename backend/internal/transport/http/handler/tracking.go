package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

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
		CFCountry string `json:"cf_country"` // Cloudflare CF-IPCountry header (production)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}

	// Use Cloudflare country header when available; fall back to ip-api.com lookup
	country := req.CFCountry
	if country == "" && req.IP != "" && req.IP != "unknown" {
		country = geoLookup(req.IP)
	}

	if err := h.trackingSvc.TrackOpen(r.Context(), req.Slug, req.IP, req.UserAgent, country); err != nil {
		// Non-critical: log but don't block proposal render
		slog.Warn("tracking: TrackOpen error", "slug", req.Slug, "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)}

// geoLookup returns the 2-letter ISO country code for an IP address via ip-api.com.
// Returns an empty string on any error (non-fatal — tracking still proceeds).
func geoLookup(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://ip-api.com/json/"+ip+"?fields=countryCode", nil)
	if err != nil {
		return ""
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return ""
	}

	var result struct {
		CountryCode string `json:"countryCode"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	return result.CountryCode
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
		defer func() { recover() }() // prevent panics from crashing the server
		for _, evt := range req.Events {
			_ = h.trackingSvc.TrackBlockTime(r.Context(), req.Slug, evt.BlockID, evt.DurationMs)
		}
	}()

	w.WriteHeader(http.StatusOK)
}
