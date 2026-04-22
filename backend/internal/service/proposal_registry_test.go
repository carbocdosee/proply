package service

// TASK-AQA-110 — Proposal Registry: service-layer integration tests
//
// AC coverage:
//   AC-1  — List returns proposals with correct fields               (integration)
//   AC-2  — Status filter: "opened" returns only opened proposals    (integration)
//   AC-3  — Title search: ILIKE match on partial keyword             (integration)
//   AC-4  — Search with SQL special chars: no error, 0 results       (integration)
//   AC-5  — Sort by created_at DESC returns newest first             (integration)
//   AC-6  — Duplicate sets title + " (копия)" suffix, status=draft   (integration)
//   AC-7  — Delete: proposal absent from List after deletion          (integration)
//   AC-8  — Soft delete: deleted_at set, not exposed via List/GetByID (integration)
//   AC-9  — Empty list: List returns 0 items for fresh user           (integration)
//
// Integration tests require TEST_DATABASE_URL. Run with:
//   TEST_DATABASE_URL=postgres://... go test ./internal/service/... -run TestRegistry

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// ─── AC-9: Empty list for a fresh user ──────────────────────────────────────

// TestRegistryList_EmptyUser verifies that a newly registered user gets an
// empty proposal list (AC-9).
func TestRegistryList_EmptyUser(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	tag := fmt.Sprintf("empty-%d", time.Now().UnixNano())
	email := fmt.Sprintf("registry-%s@example.com", tag)

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Empty User', 'free') RETURNING id`,
		email,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := NewProposalService(pool)
	result, err := svc.List(ctx, ProposalFilter{UserID: userID, Plan: "free"})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("AC-9: want 0 items for fresh user, got %d", len(result.Items))
	}
	if result.Total != 0 {
		t.Errorf("AC-9: want total=0, got %d", result.Total)
	}
}

// ─── AC-1: List returns proposals with correct fields ────────────────────────

func TestRegistryList_ReturnsProposalFields(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	tag := fmt.Sprintf("fields-%d", time.Now().UnixNano())
	email := fmt.Sprintf("registry-%s@example.com", tag)

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Fields Tester', 'free') RETURNING id`,
		email,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var proposalID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, client_name, blocks)
		 VALUES ($1, 'Fields Proposal', 'Acme Corp', '[]') RETURNING id`,
		userID,
	).Scan(&proposalID); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM proposals WHERE id=$1`, proposalID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := NewProposalService(pool)
	result, err := svc.List(ctx, ProposalFilter{UserID: userID, Plan: "free"})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("AC-1: want 1 item, got %d", len(result.Items))
	}

	p := result.Items[0]
	if p.ID != proposalID {
		t.Errorf("AC-1: id: want %q, got %q", proposalID, p.ID)
	}
	if p.Title != "Fields Proposal" {
		t.Errorf("AC-1: title: want 'Fields Proposal', got %q", p.Title)
	}
	if p.ClientName != "Acme Corp" {
		t.Errorf("AC-1: client_name: want 'Acme Corp', got %q", p.ClientName)
	}
	if p.Status == "" {
		t.Error("AC-1: status must not be empty")
	}
	if p.CreatedAt.IsZero() {
		t.Error("AC-1: created_at must be set")
	}
	if p.UpdatedAt.IsZero() {
		t.Error("AC-1: updated_at must be set")
	}
}

// ─── AC-2: Status filter ──────────────────────────────────────────────────────

func TestRegistryList_StatusFilter_Opened(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	tag := fmt.Sprintf("status-%d", time.Now().UnixNano())
	email := fmt.Sprintf("registry-%s@example.com", tag)

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Status Tester', 'free') RETURNING id`,
		email,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var draftID, openedID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, status, blocks)
		 VALUES ($1, 'Draft One', 'draft', '[]') RETURNING id`,
		userID,
	).Scan(&draftID); err != nil {
		t.Fatalf("seed draft: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, status, blocks)
		 VALUES ($1, 'Opened One', 'opened', '[]') RETURNING id`,
		userID,
	).Scan(&openedID); err != nil {
		t.Fatalf("seed opened: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM proposals WHERE id IN ($1,$2)`, draftID, openedID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := NewProposalService(pool)
	result, err := svc.List(ctx, ProposalFilter{UserID: userID, Plan: "free", Status: "opened"})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("AC-2: want total=1 for opened filter, got %d", result.Total)
	}
	if len(result.Items) != 1 {
		t.Fatalf("AC-2: want 1 item, got %d", len(result.Items))
	}
	if result.Items[0].ID != openedID {
		t.Errorf("AC-2: want openedID %q, got %q", openedID, result.Items[0].ID)
	}
	if result.Items[0].Status != "opened" {
		t.Errorf("AC-2: status: want 'opened', got %q", result.Items[0].Status)
	}
}

// ─── AC-3: Search by title keyword ───────────────────────────────────────────

func TestRegistryList_SearchByTitle(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	tag := fmt.Sprintf("search-%d", time.Now().UnixNano())
	email := fmt.Sprintf("registry-%s@example.com", tag)

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Search Tester', 'free') RETURNING id`,
		email,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var matchID, noMatchID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, blocks)
		 VALUES ($1, 'тест веб-сайт', '[]') RETURNING id`,
		userID,
	).Scan(&matchID); err != nil {
		t.Fatalf("seed match: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, blocks)
		 VALUES ($1, 'Another Project', '[]') RETURNING id`,
		userID,
	).Scan(&noMatchID); err != nil {
		t.Fatalf("seed no-match: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM proposals WHERE id IN ($1,$2)`, matchID, noMatchID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := NewProposalService(pool)
	result, err := svc.List(ctx, ProposalFilter{UserID: userID, Plan: "free", Search: "тест"})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("AC-3: want total=1, got %d", result.Total)
	}
	if len(result.Items) != 1 {
		t.Fatalf("AC-3: want 1 item, got %d", len(result.Items))
	}
	if result.Items[0].ID != matchID {
		t.Errorf("AC-3: want matchID %q, got %q", matchID, result.Items[0].ID)
	}
}

// ─── AC-4: Search with SQL special characters ─────────────────────────────────

func TestRegistryList_SearchSpecialChars_NoError(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	tag := fmt.Sprintf("special-%d", time.Now().UnixNano())
	email := fmt.Sprintf("registry-%s@example.com", tag)

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Special Tester', 'free') RETURNING id`,
		email,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var proposalID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, blocks)
		 VALUES ($1, 'Normal Proposal', '[]') RETURNING id`,
		userID,
	).Scan(&proposalID); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM proposals WHERE id=$1`, proposalID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := NewProposalService(pool)

	sqlInjections := []string{
		"' OR 1=1 --",
		`"; DROP TABLE proposals; --`,
		`%'; SELECT * FROM users WHERE '1'='1`,
		`<script>alert(1)</script>`,
		`\x00\x1f`,
	}

	for _, payload := range sqlInjections {
		result, err := svc.List(ctx, ProposalFilter{UserID: userID, Plan: "free", Search: payload})
		if err != nil {
			t.Errorf("AC-4: search(%q) returned error: %v", payload, err)
			continue
		}
		// The injection payload must not match "Normal Proposal"
		if result.Total != 0 {
			t.Errorf("AC-4: search(%q) want 0 results, got %d", payload, result.Total)
		}
	}
}

// ─── AC-5: Sort by created_at returns newest first ────────────────────────────

func TestRegistryList_SortByCreatedAt_NewestFirst(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	tag := fmt.Sprintf("sort-%d", time.Now().UnixNano())
	email := fmt.Sprintf("registry-%s@example.com", tag)

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Sort Tester', 'free') RETURNING id`,
		email,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// Insert with explicit created_at timestamps to ensure deterministic order
	var olderID, newerID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, blocks, created_at, updated_at)
		 VALUES ($1, 'Created First', '[]', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour')
		 RETURNING id`,
		userID,
	).Scan(&olderID); err != nil {
		t.Fatalf("seed older: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, blocks, created_at, updated_at)
		 VALUES ($1, 'Created Second', '[]', NOW(), NOW())
		 RETURNING id`,
		userID,
	).Scan(&newerID); err != nil {
		t.Fatalf("seed newer: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM proposals WHERE id IN ($1,$2)`, olderID, newerID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := NewProposalService(pool)

	// Sort by created_at DESC — newer item first (default order when no f.Order is set)
	result, err := svc.List(ctx, ProposalFilter{UserID: userID, Plan: "free", Sort: "created_at"})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(result.Items) < 2 {
		t.Fatalf("AC-5: want at least 2 items, got %d", len(result.Items))
	}

	// DESC: "Created Second" (newer) must come first
	if result.Items[0].ID != newerID {
		t.Errorf("AC-5: DESC sort: want newerID first, got %q (title=%q)", result.Items[0].ID, result.Items[0].Title)
	}
	if result.Items[1].ID != olderID {
		t.Errorf("AC-5: DESC sort: want olderID second, got %q (title=%q)", result.Items[1].ID, result.Items[1].Title)
	}
}

// ─── AC-6: Duplicate sets "(копия)" suffix and status=draft ──────────────────

func TestRegistryDuplicate_TitleSuffix(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	tag := fmt.Sprintf("dup-%d", time.Now().UnixNano())
	email := fmt.Sprintf("registry-%s@example.com", tag)

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Dup Tester', 'free') RETURNING id`,
		email,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var origID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, status, blocks)
		 VALUES ($1, 'Original Proposal', 'draft', '[]') RETURNING id`,
		userID,
	).Scan(&origID); err != nil {
		t.Fatalf("seed original: %v", err)
	}

	t.Cleanup(func() {
		// Use LIKE to catch both original and the copy
		_, _ = pool.Exec(ctx, `DELETE FROM proposals WHERE user_id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := NewProposalService(pool)
	copyID, err := svc.Duplicate(ctx, origID, userID)
	if err != nil {
		t.Fatalf("Duplicate error: %v", err)
	}

	// Verify title suffix
	var copyTitle, copyStatus string
	if err := pool.QueryRow(ctx,
		`SELECT title, status FROM proposals WHERE id=$1`,
		copyID,
	).Scan(&copyTitle, &copyStatus); err != nil {
		t.Fatalf("query copy: %v", err)
	}

	wantTitle := "Original Proposal (копия)"
	if copyTitle != wantTitle {
		t.Errorf("AC-6: title: want %q, got %q", wantTitle, copyTitle)
	}
	if copyStatus != "draft" {
		t.Errorf("AC-6: status: want 'draft', got %q", copyStatus)
	}
}

// ─── AC-7 / AC-8: Soft delete via service ────────────────────────────────────

// TestRegistryDelete_SoftDelete verifies that Delete sets deleted_at and the
// proposal no longer appears in List results (AC-7, AC-8).
func TestRegistryDelete_SoftDelete(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	tag := fmt.Sprintf("del-%d", time.Now().UnixNano())
	email := fmt.Sprintf("registry-%s@example.com", tag)

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Del Tester', 'free') RETURNING id`,
		email,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var proposalID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, blocks)
		 VALUES ($1, 'To Be Deleted', '[]') RETURNING id`,
		userID,
	).Scan(&proposalID); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	t.Cleanup(func() {
		// Cleanup including soft-deleted rows (which have deleted_at set)
		_, _ = pool.Exec(ctx, `DELETE FROM proposals WHERE id=$1`, proposalID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := NewProposalService(pool)

	// Confirm proposal exists before deletion
	before, err := svc.GetByID(ctx, proposalID, userID)
	if err != nil {
		t.Fatalf("GetByID before delete: %v", err)
	}
	if before.ID != proposalID {
		t.Fatalf("AC-8: unexpected id before delete: %q", before.ID)
	}

	// Delete
	if err := svc.Delete(ctx, proposalID, userID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	// AC-8a: deleted_at must be set in the DB
	var deletedAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT deleted_at FROM proposals WHERE id=$1`,
		proposalID,
	).Scan(&deletedAt); err != nil {
		t.Fatalf("query deleted_at: %v", err)
	}
	if deletedAt == nil {
		t.Error("AC-8: deleted_at must be set after Delete(), got nil")
	}

	// AC-8b: GetByID must return ErrNotFound after deletion
	_, err = svc.GetByID(ctx, proposalID, userID)
	if err != ErrNotFound {
		t.Errorf("AC-8: GetByID after delete: want ErrNotFound, got %v", err)
	}

	// AC-8c: List must not include the deleted proposal
	result, err := svc.List(ctx, ProposalFilter{UserID: userID, Plan: "free"})
	if err != nil {
		t.Fatalf("List after delete error: %v", err)
	}
	for _, p := range result.Items {
		if p.ID == proposalID {
			t.Error("AC-8: deleted proposal must not appear in List results")
		}
	}
}

// TestRegistryDelete_OtherUserCannotDelete verifies ownership check:
// deleting another user's proposal returns ErrForbidden (or ErrNotFound).
func TestRegistryDelete_OtherUserCannotDelete(t *testing.T) {
	pool := newIntegrationDB(t)
	ctx := context.Background()

	tag := fmt.Sprintf("perm-%d", time.Now().UnixNano())

	var ownerID, attackerID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Owner', 'free') RETURNING id`,
		fmt.Sprintf("owner-%s@example.com", tag),
	).Scan(&ownerID); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, plan) VALUES ($1, 'Attacker', 'free') RETURNING id`,
		fmt.Sprintf("attacker-%s@example.com", tag),
	).Scan(&attackerID); err != nil {
		t.Fatalf("seed attacker: %v", err)
	}

	var proposalID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO proposals (user_id, title, blocks)
		 VALUES ($1, 'Owner Proposal', '[]') RETURNING id`,
		ownerID,
	).Scan(&proposalID); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM proposals WHERE id=$1`, proposalID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, ownerID, attackerID)
	})

	svc := NewProposalService(pool)

	// Attacker tries to delete owner's proposal
	err := svc.Delete(ctx, proposalID, attackerID)
	if err == nil {
		t.Error("AC-8: deleting another user's proposal must return an error")
	}
	if err != ErrForbidden && err != ErrNotFound {
		t.Errorf("AC-8: want ErrForbidden or ErrNotFound, got %v", err)
	}

	// Proposal must still be retrievable by owner
	p, getErr := svc.GetByID(ctx, proposalID, ownerID)
	if getErr != nil {
		t.Fatalf("AC-8: owner GetByID after failed delete: %v", getErr)
	}
	if p.ID != proposalID {
		t.Errorf("AC-8: proposal should not be deleted, got id=%q", p.ID)
	}
}
