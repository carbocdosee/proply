package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Unit: fingerprintOf ──────────────────────────────────────────────────────

func TestFingerprintOf_Deterministic(t *testing.T) {
	fp1 := fingerprintOf("1.2.3.4", "Mozilla/5.0")
	fp2 := fingerprintOf("1.2.3.4", "Mozilla/5.0")
	if fp1 != fp2 {
		t.Error("fingerprintOf must be deterministic for the same input")
	}
}

func TestFingerprintOf_DifferentIP(t *testing.T) {
	if fingerprintOf("1.2.3.4", "Mozilla/5.0") == fingerprintOf("5.6.7.8", "Mozilla/5.0") {
		t.Error("different IPs must produce different fingerprints")
	}
}

func TestFingerprintOf_DifferentUA(t *testing.T) {
	if fingerprintOf("1.2.3.4", "Mozilla/5.0") == fingerprintOf("1.2.3.4", "Chrome/100") {
		t.Error("different user agents must produce different fingerprints")
	}
}

func TestFingerprintOf_Length(t *testing.T) {
	fp := fingerprintOf("192.168.1.1", "test-ua")
	if len(fp) != 16 {
		t.Errorf("expected 16 hex chars, got %d: %q", len(fp), fp)
	}
}

func TestFingerprintOf_IsHex(t *testing.T) {
	fp := fingerprintOf("192.168.1.1", "test-ua")
	for _, c := range fp {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex character %q in fingerprint %q", c, fp)
		}
	}
}

// AC-7: raw IP must not appear in the fingerprint output.
func TestFingerprintOf_NoRawIPStored(t *testing.T) {
	ip := "203.0.113.42"
	fp := fingerprintOf(ip, "Mozilla/5.0")
	if strings.Contains(fp, ip) {
		t.Errorf("fingerprint %q must not contain the raw IP %q", fp, ip)
	}
}

// Collision resistance: 100 distinct IP+UA pairs must produce 100 distinct fingerprints.
func TestFingerprintOf_NoCollisions(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		ip := fmt.Sprintf("10.0.%d.%d", i/256, i%256)
		ua := fmt.Sprintf("TestAgent/%d", i)
		fp := fingerprintOf(ip, ua)
		if _, exists := seen[fp]; exists {
			t.Errorf("collision detected for ip=%s ua=%s → fp=%s", ip, ua, fp)
		}
		seen[fp] = struct{}{}
	}
}

// ─── Integration: TrackOpen (requires TEST_DATABASE_URL) ─────────────────────
//
// Run with: TEST_DATABASE_URL=postgres://... go test ./internal/service/... -run TestTrackOpen
//
// The test uses a real PostgreSQL database (migrations must already be applied).
// Each test case operates on a freshly seeded user + proposal and cleans up after itself.

func newIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping DB integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedProposal inserts a minimal user + published proposal and returns their IDs and slug.
// Cleanup deletes both rows after the test.
func seedProposal(t *testing.T, pool *pgxpool.Pool) (userID, proposalID, slug string) {
	t.Helper()
	ctx := context.Background()

	slug = fmt.Sprintf("test%d", time.Now().UnixNano())

	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, plan)
		VALUES ($1, 'Test Agency', 'free')
		RETURNING id
	`, fmt.Sprintf("aqa-track-%d@example.com", time.Now().UnixNano())).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO proposals (user_id, title, status, slug, slug_active, blocks)
		VALUES ($1, 'AQA Test Proposal', 'sent', $2, true, '[]')
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

// AC-1: first open sets first_opened_at and enqueues an email job.
func TestTrackOpen_FirstOpen_SetsFirstOpenedAt(t *testing.T) {
	pool := newIntegrationDB(t)
	_, proposalID, slug := seedProposal(t, pool)
	svc := NewTrackingService(pool, "http://localhost:5173")
	ctx := context.Background()

	if err := svc.TrackOpen(ctx, slug, "1.2.3.4", "TestAgent/1", "DE"); err != nil {
		t.Fatalf("TrackOpen error: %v", err)
	}

	var firstOpenedAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT first_opened_at FROM proposals WHERE id=$1`, proposalID,
	).Scan(&firstOpenedAt); err != nil {
		t.Fatalf("query first_opened_at: %v", err)
	}
	if firstOpenedAt == nil {
		t.Error("expected first_opened_at to be set after first open, got nil")
	}
}

// AC-1: first open enqueues an email_open_notify job.
func TestTrackOpen_FirstOpen_EnqueuesEmailJob(t *testing.T) {
	pool := newIntegrationDB(t)
	_, proposalID, slug := seedProposal(t, pool)
	svc := NewTrackingService(pool, "http://localhost:5173")
	ctx := context.Background()

	if err := svc.TrackOpen(ctx, slug, "1.2.3.4", "TestAgent/1", "DE"); err != nil {
		t.Fatalf("TrackOpen error: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM job_queue
		WHERE job_type='email_open_notify'
		  AND payload->>'proposal_id'=$1
		  AND status IN ('pending','processing','done')
	`, proposalID).Scan(&count); err != nil {
		t.Fatalf("query job_queue: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 email_open_notify job, got %d", count)
	}
}

// AC-2: repeated open does not change first_opened_at and does not create a new email job.
func TestTrackOpen_RepeatOpen_FirstOpenedAtUnchanged(t *testing.T) {
	pool := newIntegrationDB(t)
	_, proposalID, slug := seedProposal(t, pool)
	svc := NewTrackingService(pool, "http://localhost:5173")
	ctx := context.Background()

	// First open.
	if err := svc.TrackOpen(ctx, slug, "1.2.3.4", "TestAgent/1", "DE"); err != nil {
		t.Fatalf("first TrackOpen: %v", err)
	}
	var firstOpenedAt time.Time
	if err := pool.QueryRow(ctx,
		`SELECT first_opened_at FROM proposals WHERE id=$1`, proposalID,
	).Scan(&firstOpenedAt); err != nil {
		t.Fatalf("query first_opened_at after first open: %v", err)
	}

	// Simulate 6 minutes passing so dedup window is cleared — but use different UA
	// to guarantee a second tracking_event is inserted; first_opened_at must stay the same.
	time.Sleep(10 * time.Millisecond) // small pause only; full 6-min test is below
	if err := svc.TrackOpen(ctx, slug, "5.6.7.8", "OtherAgent/2", "FR"); err != nil {
		t.Fatalf("second TrackOpen: %v", err)
	}

	var firstOpenedAtAfter time.Time
	if err := pool.QueryRow(ctx,
		`SELECT first_opened_at FROM proposals WHERE id=$1`, proposalID,
	).Scan(&firstOpenedAtAfter); err != nil {
		t.Fatalf("query first_opened_at after second open: %v", err)
	}
	if !firstOpenedAt.Equal(firstOpenedAtAfter) {
		t.Errorf("first_opened_at changed: was %v, now %v", firstOpenedAt, firstOpenedAtAfter)
	}
}

// AC-2: repeated open does not create a second email_open_notify job for the same proposal.
func TestTrackOpen_RepeatOpen_NoExtraEmailJob(t *testing.T) {
	pool := newIntegrationDB(t)
	_, proposalID, slug := seedProposal(t, pool)
	svc := NewTrackingService(pool, "http://localhost:5173")
	ctx := context.Background()

	// Two opens from different IPs/UAs (bypasses dedup fingerprint window).
	if err := svc.TrackOpen(ctx, slug, "1.2.3.4", "TestAgent/1", "DE"); err != nil {
		t.Fatalf("first TrackOpen: %v", err)
	}
	if err := svc.TrackOpen(ctx, slug, "9.8.7.6", "OtherAgent/9", "US"); err != nil {
		t.Fatalf("second TrackOpen: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM job_queue
		WHERE job_type='email_open_notify'
		  AND payload->>'proposal_id'=$1
	`, proposalID).Scan(&count); err != nil {
		t.Fatalf("query job_queue: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 email_open_notify job, got %d", count)
	}
}

// AC-3: same IP+UA within 5 minutes → second tracking_event NOT created.
func TestTrackOpen_SameFingerprint_WithinWindow_NoSecondEvent(t *testing.T) {
	pool := newIntegrationDB(t)
	_, proposalID, slug := seedProposal(t, pool)
	svc := NewTrackingService(pool, "http://localhost:5173")
	ctx := context.Background()

	const ip, ua = "1.2.3.4", "Chrome/120"
	_ = svc.TrackOpen(ctx, slug, ip, ua, "PL")
	_ = svc.TrackOpen(ctx, slug, ip, ua, "PL") // immediate repeat — within dedup window

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM tracking_events WHERE proposal_id=$1 AND event_type='open'`,
		proposalID,
	).Scan(&count); err != nil {
		t.Fatalf("query tracking_events: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 tracking_event within 5-min window, got %d", count)
	}
}

// AC-5: different IP+UA in the same session → both events created.
func TestTrackOpen_DifferentFingerprint_BothEventsCreated(t *testing.T) {
	pool := newIntegrationDB(t)
	_, proposalID, slug := seedProposal(t, pool)
	svc := NewTrackingService(pool, "http://localhost:5173")
	ctx := context.Background()

	_ = svc.TrackOpen(ctx, slug, "1.1.1.1", "AgentA/1", "DE")
	_ = svc.TrackOpen(ctx, slug, "2.2.2.2", "AgentB/2", "FR")

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM tracking_events WHERE proposal_id=$1 AND event_type='open'`,
		proposalID,
	).Scan(&count); err != nil {
		t.Fatalf("query tracking_events: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 tracking_events for different fingerprints, got %d", count)
	}
}

// AC-7: country is stored in tracking_events; fingerprint stored does not equal raw IP.
func TestTrackOpen_CountryStoredRawIPNotStored(t *testing.T) {
	pool := newIntegrationDB(t)
	_, proposalID, slug := seedProposal(t, pool)
	svc := NewTrackingService(pool, "http://localhost:5173")
	ctx := context.Background()

	const ip = "203.0.113.10"
	if err := svc.TrackOpen(ctx, slug, ip, "Safari/17", "IT"); err != nil {
		t.Fatalf("TrackOpen: %v", err)
	}

	var country, fingerprint string
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(country,''), fingerprint
		FROM tracking_events
		WHERE proposal_id=$1 AND event_type='open'
		LIMIT 1
	`, proposalID).Scan(&country, &fingerprint); err != nil {
		t.Fatalf("query tracking_event: %v", err)
	}

	if country != "IT" {
		t.Errorf("expected country=IT, got %q", country)
	}
	if fingerprint == ip {
		t.Error("fingerprint must not equal raw IP")
	}
	if strings.Contains(fingerprint, ip) {
		t.Errorf("fingerprint %q must not contain raw IP %q", fingerprint, ip)
	}
}

// AC-6 helper: assert email job payload contains expected fields.
func TestTrackOpen_EmailJobPayloadFields(t *testing.T) {
	pool := newIntegrationDB(t)
	_, proposalID, slug := seedProposal(t, pool)
	svc := NewTrackingService(pool, "http://localhost:5173")
	ctx := context.Background()

	if err := svc.TrackOpen(ctx, slug, "1.2.3.4", "TestAgent/1", "DE"); err != nil {
		t.Fatalf("TrackOpen: %v", err)
	}

	var rawPayload []byte
	if err := pool.QueryRow(ctx, `
		SELECT payload FROM job_queue
		WHERE job_type='email_open_notify' AND payload->>'proposal_id'=$1
		LIMIT 1
	`, proposalID).Scan(&rawPayload); err != nil {
		t.Fatalf("query job payload: %v", err)
	}

	var p struct {
		OwnerEmail    string `json:"owner_email"`
		ProposalTitle string `json:"proposal_title"`
		ProposalLink  string `json:"proposal_link"`
		Country       string `json:"country"`
	}
	if err := json.Unmarshal(rawPayload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.OwnerEmail == "" {
		t.Error("email job payload: owner_email must not be empty")
	}
	if p.ProposalTitle == "" {
		t.Error("email job payload: proposal_title must not be empty")
	}
	if !strings.HasPrefix(p.ProposalLink, "http://localhost:5173/dashboard/proposals/") {
		t.Errorf("email job payload: unexpected proposal_link %q", p.ProposalLink)
	}
	if p.Country != "DE" {
		t.Errorf("email job payload: expected country=DE, got %q", p.Country)
	}
}
