// TASK-AQA-105 — Publish / Revoke / GetPublic handler unit tests
//
// AC coverage (handler layer, no DB required):
//   AC1  — Publish → 401 when no auth
//   AC1  — Publish → 403 when email not verified  (already in proposal_handler_test.go)
//   AC5  — Revoke  → 401 when no auth             (already in proposal_handler_test.go)
//   AC5  — Revoke  → 403 when email not verified  (handler doesn't enforce this — no gate)
//
// AC6 — Open question / discrepancy:
//   The AC states: POST /publish for an already-published proposal → idempotent (200, same slug).
//   The actual implementation (ProposalService.Publish) returns ErrConflict when the
//   proposal already has a slug, and the handler maps that to 409 ALREADY_PUBLISHED.
//   Decision: test documents the REAL behavior (409) and flags the discrepancy for PO/BA review.
//   Integration verification requires a real DB (see e2e/publish.spec.ts for browser-level AC6).
//
// AC3 — 100 parallel publishes → unique slugs
//   Covered at the slug-generation layer in pkg/slug/slug_test.go
//   (TestGenerate_Concurrent_100_UniqueSlugs). Full DB-level integration test requires
//   the test environment from docker-compose.test.yml.

package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"proply/internal/transport/http/handler"
)

// ─── Public handler: GetProposal ─────────────────────────────────────────────

func newTestPublicHandler() *handler.PublicHandler {
	return handler.NewPublicHandler(nil)
}

// TestGetPublicProposal_NilSlugParam verifies the handler does not panic when
// the slug URL param is empty (Chi not wired — empty string passed to service).
// With nil service it will panic after the slug extraction → tested at E2E level.
// This test just confirms the handler can be constructed without a real service.
func TestPublicHandler_CanBeConstructed(t *testing.T) {
	h := newTestPublicHandler()
	if h == nil {
		t.Fatal("NewPublicHandler returned nil")
	}
}

// ─── Publish: additional coverage for verified-user gate paths ───────────────

// TestPublishProposal_VerifiedUser_NilServicePanic confirms that once the
// auth and email-verification gates are cleared the handler calls the service.
// Because ProposalService is a concrete struct (not an interface), we cannot
// inject a mock — nil service causes a panic (expected). This documents that the
// handler delegates to the service after gate checks, validated via panic recovery.
func TestPublishProposal_VerifiedUser_DelegatestoService(t *testing.T) {
	h := handler.NewProposalHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proposals/some-id/publish", nil)
	req = withVerifiedFakeClaims(req)
	w := httptest.NewRecorder()

	// The handler will panic trying to call nil.Publish — recover and assert
	// it got past both auth gates (i.e. the panic proves gate checks passed).
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic from nil ProposalService after passing auth gates, got none")
		}
	}()

	h.Publish(w, req)
}

// ─── Revoke: verified user gate ──────────────────────────────────────────────

// TestRevokeProposal_VerifiedUser_DelegatestoService mirrors the Publish test above:
// once auth is cleared, the nil service causes a panic (expected).
func TestRevokeProposal_VerifiedUser_DelegatestoService(t *testing.T) {
	h := handler.NewProposalHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proposals/some-id/revoke", nil)
	req = withVerifiedFakeClaims(req)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic from nil ProposalService, got none — auth gate may have blocked early")
		}
	}()

	h.Revoke(w, req)
}

// ─── PublicHandler: Approve ──────────────────────────────────────────────────

// TestApprove_InvalidJSON verifies the Approve handler returns 400 INVALID_JSON
// before calling the service (safe with nil service).
func TestApprove_InvalidJSON(t *testing.T) {
	h := newTestPublicHandler()

	// Simulate Chi URL param by wrapping the request with a fake chi context.
	// Approve reads the slug from URL param, but with nil service it would panic —
	// we test only the JSON decoding gate here (empty body → decode error).
	// Chi param extraction returns "" for un-wired requests, which is fine
	// because the JSON decode error fires first.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/proposals/some-slug/approve", nil)
	w := httptest.NewRecorder()
	h.Approve(w, req)

	// Empty body → json.Decoder returns io.EOF → 400 INVALID_JSON
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}
