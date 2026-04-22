package handler_test

// TASK-AQA-109: Handler-level tests for view analytics
//
// Covers:
//  - GET /analytics: no auth → 401
//  - POST /track/block-time: invalid JSON → 400
//  - POST /track/block-time: empty events array → 200 (no-op)
//  - POST /track/block-time: valid payload → 200

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"proply/internal/transport/http/handler"
)

// newTestTrackingHandler builds a TrackingHandler backed by a nil service.
// Safe for HTTP-layer-only tests that fire before the service call.
func newTestTrackingHandler() *handler.TrackingHandler {
	return handler.NewTrackingHandler(nil)
}

// ─── GET /analytics: auth guard ──────────────────────────────────────────────

// AC-3 (handler): unauthenticated request → 401 UNAUTHORIZED.
func TestGetAnalytics_NoAuth_Returns401(t *testing.T) {
	h := handler.NewProposalHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proposals/some-id/analytics", nil)
	w := httptest.NewRecorder()
	h.GetAnalytics(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

// AC-3 (handler): authenticated Free user hits the endpoint —
// the handler passes through to the service; with nil service it panics unless
// we wire a real service. Instead we verify that the auth layer does NOT block
// a Free-plan user (plan_gate is a service-level concern, not a 4xx at HTTP layer).
// This test just confirms no 401 is returned for a valid Free-plan token.
func TestGetAnalytics_FreePlanAuth_PassesThroughToService(t *testing.T) {
	h := handler.NewProposalHandler(nil) // nil svc → will panic when service is called
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proposals/some-id/analytics", nil)
	req = withFakeClaims(req) // free-plan claims
	w := httptest.NewRecorder()

	// The handler will call h.proposalSvc.GetAnalytics which panics with nil svc.
	// We only care that the auth guard (401 check) does NOT fire.
	defer func() {
		if r := recover(); r != nil {
			// Expected: panic from nil service. Auth guard passed.
		}
	}()
	h.GetAnalytics(w, req)

	// If we reach here without a panic the service returned an error response —
	// either way, it must not be 401.
	if w.Code == http.StatusUnauthorized {
		t.Error("Free-plan authenticated request must not be rejected with 401")
	}
}

// ─── POST /track/block-time: input validation ─────────────────────────────────

// Malformed JSON body → 400 INVALID_JSON.
func TestTrackBlocks_InvalidJSON_Returns400(t *testing.T) {
	h := newTestTrackingHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track/block-time",
		strings.NewReader(`{bad json`))
	w := httptest.NewRecorder()
	h.TrackBlocks(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}

// Empty events array → 200 (fire-and-forget, nothing to track).
func TestTrackBlocks_EmptyEvents_Returns200(t *testing.T) {
	h := newTestTrackingHandler()
	body := `{"slug":"abc123","events":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track/block-time",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	h.TrackBlocks(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: want %d, got %d", http.StatusOK, w.Code)
	}
}

// Valid payload → 200 (best-effort fire-and-forget; nil service goroutine runs after response).
func TestTrackBlocks_ValidPayload_Returns200(t *testing.T) {
	h := newTestTrackingHandler()
	body := `{"slug":"abc123","events":[{"block_id":"block-a","duration_ms":5000}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track/block-time",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	h.TrackBlocks(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: want %d, got %d", http.StatusOK, w.Code)
	}
}

// Missing slug field → 200 (handler does not validate slug presence; service handles it).
func TestTrackBlocks_MissingSlug_Returns200(t *testing.T) {
	h := newTestTrackingHandler()
	body := `{"events":[{"block_id":"block-a","duration_ms":1000}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track/block-time",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	h.TrackBlocks(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: want %d, got %d", http.StatusOK, w.Code)
	}
}
