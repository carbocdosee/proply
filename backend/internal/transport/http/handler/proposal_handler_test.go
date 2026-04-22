package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"proply/internal/transport/http/handler"
)

// newTestProposalHandler returns a ProposalHandler with nil service.
// Safe for testing HTTP-layer paths that execute before the service call.
func newTestProposalHandler() *handler.ProposalHandler {
	return handler.NewProposalHandler(nil)
}

// ─── ListTemplates ────────────────────────────────────────────────────────────

// The endpoint is public — no auth token required.
func TestListTemplates_NoAuthRequired(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	w := httptest.NewRecorder()
	h.ListTemplates(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: want %d, got %d", http.StatusOK, w.Code)
	}
}

func TestListTemplates_ReturnsFiveTemplates(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	w := httptest.NewRecorder()
	h.ListTemplates(w, req)

	var body []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	const wantCount = 5
	if len(body) != wantCount {
		t.Errorf("template count: want %d, got %d", wantCount, len(body))
	}
}

func TestListTemplates_EachTemplateHasRequiredFields(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	w := httptest.NewRecorder()
	h.ListTemplates(w, req)

	var body []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	for i, tpl := range body {
		for _, field := range []string{"id", "name", "description", "block_types"} {
			if v, ok := tpl[field]; !ok || v == nil || v == "" {
				t.Errorf("template[%d]: missing or empty field %q", i, field)
			}
		}
		if types, ok := tpl["block_types"].([]any); !ok || len(types) == 0 {
			t.Errorf("template[%d]: block_types must be a non-empty array", i)
		}
	}
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestCreateProposal_NoAuth(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proposals",
		strings.NewReader(`{"title":"Test proposal"}`))
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

func TestCreateProposal_InvalidJSON(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proposals",
		strings.NewReader(`{bad json`))
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestListProposals_NoAuth(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proposals", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

// ─── Update ───────────────────────────────────────────────────────────────────

func TestUpdateProposal_NoAuth(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/proposals/some-id",
		strings.NewReader(`{"title":"Updated"}`))
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

func TestUpdateProposal_InvalidJSON(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/proposals/some-id",
		strings.NewReader(`{bad`))
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}

// ─── Publish ──────────────────────────────────────────────────────────────────

func TestPublishProposal_NoAuth(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proposals/some-id/publish", nil)
	w := httptest.NewRecorder()
	h.Publish(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

// Publish requires a verified email — the check fires before the service call,
// so the test is safe with a nil ProposalService.
func TestPublishProposal_EmailNotVerified(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proposals/some-id/publish", nil)
	// withFakeClaims sets EmailVerified: false
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.Publish(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: want %d (Forbidden), got %d", http.StatusForbidden, w.Code)
	}
	assertErrorCode(t, w, "EMAIL_NOT_VERIFIED")
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func TestGetProposal_NoAuth(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proposals/some-id", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestDeleteProposal_NoAuth(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/proposals/some-id", nil)
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

// ─── Duplicate ────────────────────────────────────────────────────────────────

func TestDuplicateProposal_NoAuth(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proposals/some-id/duplicate", nil)
	w := httptest.NewRecorder()
	h.Duplicate(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

// ─── Revoke ───────────────────────────────────────────────────────────────────

func TestRevokeProposal_NoAuth(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proposals/some-id/revoke", nil)
	w := httptest.NewRecorder()
	h.Revoke(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

// ─── UpdateStatus ─────────────────────────────────────────────────────────────

func TestUpdateProposalStatus_NoAuth(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/proposals/some-id/status",
		strings.NewReader(`{"status":"sent"}`))
	w := httptest.NewRecorder()
	h.UpdateStatus(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

func TestUpdateProposalStatus_InvalidJSON(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/proposals/some-id/status",
		strings.NewReader(`{bad json`))
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.UpdateStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}

// ─── GetAnalytics ─────────────────────────────────────────────────────────────

func TestGetAnalytics_NoAuth(t *testing.T) {
	h := newTestProposalHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proposals/some-id/analytics", nil)
	w := httptest.NewRecorder()
	h.GetAnalytics(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}
