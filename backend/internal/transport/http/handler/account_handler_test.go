package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"proply/internal/transport/http/handler"
)

// newTestAccountHandler builds an AccountHandler with nil service —
// sufficient for testing HTTP-layer paths that execute before the service call.
func newTestAccountHandler() *handler.AccountHandler {
	return handler.NewAccountHandler(nil)
}

// ─── UpdateBranding ───────────────────────────────────────────────────────────

func TestUpdateBranding_NoAuth(t *testing.T) {
	h := newTestAccountHandler()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/branding",
		strings.NewReader(`{"primary_color":"#FF0000"}`))
	w := httptest.NewRecorder()
	h.UpdateBranding(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

func TestUpdateBranding_InvalidJSON(t *testing.T) {
	h := newTestAccountHandler()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/branding",
		strings.NewReader(`{bad json`))
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.UpdateBranding(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}

// ─── UpdateProfile ────────────────────────────────────────────────────────────

func TestUpdateProfile_NoAuth(t *testing.T) {
	h := newTestAccountHandler()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account",
		strings.NewReader(`{"name":"Alice"}`))
	w := httptest.NewRecorder()
	h.UpdateProfile(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

func TestUpdateProfile_InvalidJSON(t *testing.T) {
	h := newTestAccountHandler()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account",
		strings.NewReader(`{bad`))
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.UpdateProfile(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}

// ─── UpdateRetention ─────────────────────────────────────────────────────────

func TestUpdateRetention_NoAuth(t *testing.T) {
	h := newTestAccountHandler()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/retention",
		strings.NewReader(`{"months":12}`))
	w := httptest.NewRecorder()
	h.UpdateRetention(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestUpdateRetention_InvalidJSON(t *testing.T) {
	h := newTestAccountHandler()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/retention",
		strings.NewReader(`{invalid}`))
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.UpdateRetention(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}

// ─── ExportData / DeleteAccount ───────────────────────────────────────────────

func TestExportData_NoAuth(t *testing.T) {
	h := newTestAccountHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/export", nil)
	w := httptest.NewRecorder()
	h.ExportData(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestDeleteAccount_NoAuth(t *testing.T) {
	h := newTestAccountHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/account", nil)
	w := httptest.NewRecorder()
	h.DeleteAccount(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// ─── Presign (UploadHandler) ──────────────────────────────────────────────────

func TestPresign_NoAuth(t *testing.T) {
	h := newTestUploadHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/presign",
		strings.NewReader(`{"file_type":"logo","content_type":"image/png","size_bytes":1024}`))
	w := httptest.NewRecorder()
	h.Presign(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want %d, got %d", http.StatusUnauthorized, w.Code)
	}
	assertErrorCode(t, w, "UNAUTHORIZED")
}

func TestPresign_InvalidJSON(t *testing.T) {
	h := newTestUploadHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/presign",
		strings.NewReader(`{bad json`))
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.Presign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "INVALID_JSON")
}

func TestPresign_MissingFileType(t *testing.T) {
	h := newTestUploadHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/presign",
		strings.NewReader(`{"content_type":"image/png","size_bytes":1024}`))
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.Presign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "MISSING_FIELDS")
}

func TestPresign_MissingContentType(t *testing.T) {
	h := newTestUploadHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/presign",
		strings.NewReader(`{"file_type":"logo","size_bytes":1024}`))
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.Presign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "MISSING_FIELDS")
}

func TestPresign_ZeroSizeBytes(t *testing.T) {
	h := newTestUploadHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/presign",
		strings.NewReader(`{"file_type":"logo","content_type":"image/png","size_bytes":0}`))
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.Presign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want %d, got %d", http.StatusBadRequest, w.Code)
	}
	assertErrorCode(t, w, "MISSING_FIELDS")
}

// TestPresign_StorageNotConfigured verifies the 503 path when S3 env vars are absent.
func TestPresign_StorageNotConfigured(t *testing.T) {
	h := newUnconfiguredStorageHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/presign",
		strings.NewReader(`{"file_type":"logo","content_type":"image/png","size_bytes":1024}`))
	req = withFakeClaims(req)
	w := httptest.NewRecorder()
	h.Presign(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status: want %d (ServiceUnavailable), got %d", http.StatusServiceUnavailable, w.Code)
	}
	assertErrorCode(t, w, "STORAGE_NOT_CONFIGURED")
}

// newTestUploadHandler builds an UploadHandler with nil storage service —
// used when testing HTTP-layer paths that fire before the service is called.
func newTestUploadHandler() *handler.UploadHandler {
	return handler.NewUploadHandler(nil)
}
