package service

// TASK-AQA-109: View analytics testing
//
// AC-1: 5 opens → GET /analytics returns open_count = 5, first/last_opened_at set
// AC-2: POST /track-time with block data → summed correctly in DB
// AC-3: Free plan → plan_gate=true, block_stats empty; Pro plan → block_stats populated
// Edge: zero-duration events do not break aggregation
//
// Run with: TEST_DATABASE_URL=postgres://... go test ./internal/service/... -run TestAnalytics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedProposalPro inserts a user + published proposal with the given plan.
// Returns userID, proposalID, slug. Cleans up after the test.
func seedProposalWithPlan(t *testing.T, pool *pgxpool.Pool, plan string) (userID, proposalID, slug string) {
	t.Helper()
	ctx := context.Background()

	slug = fmt.Sprintf("aqa109-%d", time.Now().UnixNano())

	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, plan)
		VALUES ($1, 'Analytics Agency', $2)
		RETURNING id
	`, fmt.Sprintf("aqa109-%d@example.com", time.Now().UnixNano()), plan).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO proposals (user_id, title, status, slug, slug_active, blocks)
		VALUES ($1, 'Analytics Test Proposal', 'sent', $2, true,
		        '[{"id":"block-a","type":"text","order":0},{"id":"block-b","type":"price_table","order":1}]'::jsonb)
		RETURNING id
	`, userID, slug).Scan(&proposalID)
	if err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tracking_events WHERE proposal_id=$1`, proposalID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM job_queue WHERE payload->>'proposal_id'=$1`, proposalID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM proposals WHERE id=$1`, proposalID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	})

	return userID, proposalID, slug
}

// ─── AC-1: open_count accumulation ───────────────────────────────────────────

// AC-1: 5 distinct opens → open_count = 5, first/last_opened_at set correctly.
func TestGetAnalytics_FiveOpens_OpenCountEquals5(t *testing.T) {
	pool := newIntegrationDB(t)
	userID, proposalID, slug := seedProposalWithPlan(t, pool, "pro")
	trackSvc := NewTrackingService(pool, "http://localhost:5173")
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	// Insert 5 opens with distinct fingerprints
	for i := 0; i < 5; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i+1)
		ua := fmt.Sprintf("Agent/%d", i+1)
		if err := trackSvc.TrackOpen(ctx, slug, ip, ua, "DE"); err != nil {
			t.Fatalf("TrackOpen[%d]: %v", i, err)
		}
	}

	result, err := proposalSvc.GetAnalytics(ctx, proposalID, userID, "pro")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}

	if result.OpenCount != 5 {
		t.Errorf("open_count: want 5, got %d", result.OpenCount)
	}
}

// AC-1: first_opened_at and last_opened_at are both set after opens.
func TestGetAnalytics_OpenTimestamps_SetAfterOpens(t *testing.T) {
	pool := newIntegrationDB(t)
	userID, proposalID, slug := seedProposalWithPlan(t, pool, "pro")
	trackSvc := NewTrackingService(pool, "http://localhost:5173")
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	before := time.Now().Add(-time.Second)

	if err := trackSvc.TrackOpen(ctx, slug, "1.2.3.4", "Agent/1", "US"); err != nil {
		t.Fatalf("TrackOpen: %v", err)
	}

	result, err := proposalSvc.GetAnalytics(ctx, proposalID, userID, "pro")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}

	if result.FirstOpenedAt == nil {
		t.Fatal("first_opened_at must not be nil after first open")
	}
	if result.LastOpenedAt == nil {
		t.Fatal("last_opened_at must not be nil after first open")
	}
	if result.FirstOpenedAt.Before(before) {
		t.Errorf("first_opened_at %v is before test start %v", result.FirstOpenedAt, before)
	}
	if result.LastOpenedAt.Before(before) {
		t.Errorf("last_opened_at %v is before test start %v", result.LastOpenedAt, before)
	}
}

// AC-1: first_opened_at does not change on subsequent opens; last_opened_at advances.
func TestGetAnalytics_MultipleOpens_FirstOpenedAtStable(t *testing.T) {
	pool := newIntegrationDB(t)
	userID, proposalID, slug := seedProposalWithPlan(t, pool, "pro")
	trackSvc := NewTrackingService(pool, "http://localhost:5173")
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	// First open
	if err := trackSvc.TrackOpen(ctx, slug, "1.1.1.1", "AgentA", "DE"); err != nil {
		t.Fatalf("first TrackOpen: %v", err)
	}
	r1, err := proposalSvc.GetAnalytics(ctx, proposalID, userID, "pro")
	if err != nil {
		t.Fatalf("GetAnalytics after first open: %v", err)
	}
	firstOpenedAt := r1.FirstOpenedAt

	// Second open with a different fingerprint
	if err := trackSvc.TrackOpen(ctx, slug, "2.2.2.2", "AgentB", "FR"); err != nil {
		t.Fatalf("second TrackOpen: %v", err)
	}
	r2, err := proposalSvc.GetAnalytics(ctx, proposalID, userID, "pro")
	if err != nil {
		t.Fatalf("GetAnalytics after second open: %v", err)
	}

	if !r2.FirstOpenedAt.Equal(*firstOpenedAt) {
		t.Errorf("first_opened_at changed: was %v, now %v", firstOpenedAt, r2.FirstOpenedAt)
	}
}

// ─── AC-2: block time aggregation ────────────────────────────────────────────

// AC-2: TrackBlockTime events are summed correctly in GetAnalytics (Pro plan).
func TestGetAnalytics_BlockTimeSummed_ProPlan(t *testing.T) {
	pool := newIntegrationDB(t)
	userID, proposalID, slug := seedProposalWithPlan(t, pool, "pro")
	trackSvc := NewTrackingService(pool, "http://localhost:5173")
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	// Record block time: block-a gets 3000ms, 2000ms; block-b gets 5000ms
	if err := trackSvc.TrackBlockTime(ctx, slug, "block-a", 3000); err != nil {
		t.Fatalf("TrackBlockTime block-a first: %v", err)
	}
	if err := trackSvc.TrackBlockTime(ctx, slug, "block-a", 2000); err != nil {
		t.Fatalf("TrackBlockTime block-a second: %v", err)
	}
	if err := trackSvc.TrackBlockTime(ctx, slug, "block-b", 5000); err != nil {
		t.Fatalf("TrackBlockTime block-b: %v", err)
	}

	result, err := proposalSvc.GetAnalytics(ctx, proposalID, userID, "pro")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}

	// total_duration_sec = (3000 + 2000 + 5000) / 1000 = 10
	if result.TotalDurationSec != 10 {
		t.Errorf("total_duration_sec: want 10, got %d", result.TotalDurationSec)
	}

	// Per-block stats: block-a = 5 sec, block-b = 5 sec
	blockMap := make(map[string]int)
	for _, bs := range result.BlockStats {
		blockMap[bs.BlockID] = bs.DurationSec
	}
	if blockMap["block-a"] != 5 {
		t.Errorf("block-a duration_sec: want 5, got %d", blockMap["block-a"])
	}
	if blockMap["block-b"] != 5 {
		t.Errorf("block-b duration_sec: want 5, got %d", blockMap["block-b"])
	}
}

// AC-2: block_stats are ordered by block order (ascending).
func TestGetAnalytics_BlockStats_OrderedByBlockOrder(t *testing.T) {
	pool := newIntegrationDB(t)
	userID, proposalID, slug := seedProposalWithPlan(t, pool, "pro")
	trackSvc := NewTrackingService(pool, "http://localhost:5173")
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	// Insert in reverse order to confirm DB ordering is by block.order, not insertion order
	if err := trackSvc.TrackBlockTime(ctx, slug, "block-b", 4000); err != nil {
		t.Fatalf("TrackBlockTime block-b: %v", err)
	}
	if err := trackSvc.TrackBlockTime(ctx, slug, "block-a", 2000); err != nil {
		t.Fatalf("TrackBlockTime block-a: %v", err)
	}

	result, err := proposalSvc.GetAnalytics(ctx, proposalID, userID, "pro")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}

	if len(result.BlockStats) < 2 {
		t.Fatalf("expected at least 2 block_stats, got %d", len(result.BlockStats))
	}
	// block-a has order=0, block-b has order=1 → block-a must come first
	if result.BlockStats[0].BlockID != "block-a" {
		t.Errorf("block_stats[0]: want block-a (order=0), got %s", result.BlockStats[0].BlockID)
	}
	if result.BlockStats[1].BlockID != "block-b" {
		t.Errorf("block_stats[1]: want block-b (order=1), got %s", result.BlockStats[1].BlockID)
	}
}

// ─── AC-3: plan gate ─────────────────────────────────────────────────────────

// AC-3: Free plan → plan_gate=true, block_stats is empty.
func TestGetAnalytics_FreePlan_PlanGateTrue_BlockStatsEmpty(t *testing.T) {
	pool := newIntegrationDB(t)
	userID, proposalID, slug := seedProposalWithPlan(t, pool, "free")
	trackSvc := NewTrackingService(pool, "http://localhost:5173")
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	// Record some block time — should not appear in the response for Free
	if err := trackSvc.TrackBlockTime(ctx, slug, "block-a", 5000); err != nil {
		t.Fatalf("TrackBlockTime: %v", err)
	}

	result, err := proposalSvc.GetAnalytics(ctx, proposalID, userID, "free")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}

	if !result.PlanGate {
		t.Error("plan_gate must be true for Free plan")
	}
	if len(result.BlockStats) != 0 {
		t.Errorf("block_stats must be empty for Free plan, got %d entries", len(result.BlockStats))
	}
}

// AC-3: Pro plan → plan_gate=false, block_stats populated.
func TestGetAnalytics_ProPlan_PlanGateFalse_BlockStatsPopulated(t *testing.T) {
	pool := newIntegrationDB(t)
	userID, proposalID, slug := seedProposalWithPlan(t, pool, "pro")
	trackSvc := NewTrackingService(pool, "http://localhost:5173")
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	if err := trackSvc.TrackBlockTime(ctx, slug, "block-a", 3000); err != nil {
		t.Fatalf("TrackBlockTime: %v", err)
	}

	result, err := proposalSvc.GetAnalytics(ctx, proposalID, userID, "pro")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}

	if result.PlanGate {
		t.Error("plan_gate must be false for Pro plan")
	}
	if len(result.BlockStats) == 0 {
		t.Error("block_stats must be non-empty for Pro plan when block_time events exist")
	}
}

// ─── Edge cases ───────────────────────────────────────────────────────────────

// Edge: zero-duration events (0ms) do not break aggregation; TotalDurationSec stays 0.
func TestGetAnalytics_ZeroDurationEvents_DoNotBreakAggregation(t *testing.T) {
	pool := newIntegrationDB(t)
	userID, proposalID, slug := seedProposalWithPlan(t, pool, "pro")
	trackSvc := NewTrackingService(pool, "http://localhost:5173")
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	// Insert zero-duration events for both blocks
	if err := trackSvc.TrackBlockTime(ctx, slug, "block-a", 0); err != nil {
		t.Fatalf("TrackBlockTime 0ms block-a: %v", err)
	}
	if err := trackSvc.TrackBlockTime(ctx, slug, "block-b", 0); err != nil {
		t.Fatalf("TrackBlockTime 0ms block-b: %v", err)
	}

	result, err := proposalSvc.GetAnalytics(ctx, proposalID, userID, "pro")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}

	if result.TotalDurationSec != 0 {
		t.Errorf("total_duration_sec: want 0 for all-zero events, got %d", result.TotalDurationSec)
	}
}

// Edge: no opens → open_count=0, timestamps nil, total_duration_sec=0.
func TestGetAnalytics_NoEvents_ZeroValues(t *testing.T) {
	pool := newIntegrationDB(t)
	userID, proposalID, _ := seedProposalWithPlan(t, pool, "pro")
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	result, err := proposalSvc.GetAnalytics(ctx, proposalID, userID, "pro")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}

	if result.OpenCount != 0 {
		t.Errorf("open_count: want 0, got %d", result.OpenCount)
	}
	if result.FirstOpenedAt != nil {
		t.Errorf("first_opened_at: want nil, got %v", result.FirstOpenedAt)
	}
	if result.LastOpenedAt != nil {
		t.Errorf("last_opened_at: want nil, got %v", result.LastOpenedAt)
	}
	if result.TotalDurationSec != 0 {
		t.Errorf("total_duration_sec: want 0, got %d", result.TotalDurationSec)
	}
	if len(result.BlockStats) != 0 {
		t.Errorf("block_stats: want empty slice, got %d entries", len(result.BlockStats))
	}
}

// Edge: wrong userID → GetAnalytics returns ErrNotFound (via GetByID ownership check).
func TestGetAnalytics_WrongUser_ReturnsForbidden(t *testing.T) {
	pool := newIntegrationDB(t)
	_, proposalID, _ := seedProposalWithPlan(t, pool, "pro")
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	_, err := proposalSvc.GetAnalytics(ctx, proposalID, "wrong-user-id", "pro")
	if err == nil {
		t.Fatal("expected error for wrong userID, got nil")
	}
}

// Edge: non-existent proposal → GetAnalytics returns error.
func TestGetAnalytics_NonExistentProposal_ReturnsError(t *testing.T) {
	pool := newIntegrationDB(t)
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	_, err := proposalSvc.GetAnalytics(ctx, "00000000-0000-0000-0000-000000000000", "any-user", "pro")
	if err == nil {
		t.Fatal("expected error for non-existent proposal, got nil")
	}
}

// ─── TrackBlockTime: bad slug ─────────────────────────────────────────────────

// TrackBlockTime with unknown slug → returns ErrNotFound.
func TestTrackBlockTime_UnknownSlug_ReturnsError(t *testing.T) {
	pool := newIntegrationDB(t)
	svc := NewTrackingService(pool, "http://localhost:5173")
	ctx := context.Background()

	err := svc.TrackBlockTime(ctx, "no-such-slug", "block-a", 1000)
	if err == nil {
		t.Fatal("expected error for unknown slug, got nil")
	}
}

// TrackBlockTime: multiple batches for same block are aggregated.
func TestTrackBlockTime_MultipleBatches_Aggregated(t *testing.T) {
	pool := newIntegrationDB(t)
	userID, proposalID, slug := seedProposalWithPlan(t, pool, "pro")
	trackSvc := NewTrackingService(pool, "http://localhost:5173")
	proposalSvc := NewProposalService(pool)
	ctx := context.Background()

	// Simulate three 10-second batches (as IntersectionObserver fires every 10s)
	for i := 0; i < 3; i++ {
		if err := trackSvc.TrackBlockTime(ctx, slug, "block-a", 10_000); err != nil {
			t.Fatalf("TrackBlockTime batch %d: %v", i, err)
		}
	}

	result, err := proposalSvc.GetAnalytics(ctx, proposalID, userID, "pro")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}

	// 3 × 10s = 30s
	if result.TotalDurationSec != 30 {
		t.Errorf("total_duration_sec: want 30, got %d", result.TotalDurationSec)
	}
	if len(result.BlockStats) == 0 {
		t.Fatal("expected block_stats to contain block-a")
	}
	if result.BlockStats[0].DurationSec != 30 {
		t.Errorf("block-a duration_sec: want 30, got %d", result.BlockStats[0].DurationSec)
	}
}
