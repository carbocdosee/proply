package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// ─── Mock EmailSender ─────────────────────────────────────────────────────────

type mockEmailSender struct {
	calls []mockEmailCall
}

type mockEmailCall struct {
	to, subject, html string
}

func (m *mockEmailSender) Send(_ context.Context, to, subject, html string) error {
	m.calls = append(m.calls, mockEmailCall{to: to, subject: subject, html: html})
	return nil
}

func (m *mockEmailSender) reset() { m.calls = nil }

// ─── AC-6: job_queue processed → EmailSender called with correct params ───────

// TestWorker_EmailOpenNotify verifies that handleEmailOpenNotify sends an email
// to the proposal owner with the correct subject and non-empty body.
func TestWorker_EmailOpenNotify_SendsToOwner(t *testing.T) {
	sender := &mockEmailSender{}
	w := &Worker{emailSender: sender}

	payload, _ := json.Marshal(map[string]string{
		"owner_email":    "owner@example.com",
		"proposal_title": "My Proposal",
		"client_name":    "Acme Corp",
		"proposal_link":  "http://localhost:5173/dashboard/proposals/abc123",
		"country":        "DE",
	})

	if err := w.handleEmailOpenNotify(context.Background(), payload); err != nil {
		t.Fatalf("handleEmailOpenNotify error: %v", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(sender.calls))
	}

	call := sender.calls[0]
	if call.to != "owner@example.com" {
		t.Errorf("expected to=owner@example.com, got %q", call.to)
	}
	// Subject must reference the client name (display name).
	if !strings.Contains(call.subject, "Acme Corp") {
		t.Errorf("expected subject to contain client name, got %q", call.subject)
	}
	if call.html == "" {
		t.Error("email body must not be empty")
	}
}

// TestWorker_EmailOpenNotify_ContainsProposalLink verifies that the proposal
// dashboard link is present in the email body.
func TestWorker_EmailOpenNotify_ContainsProposalLink(t *testing.T) {
	sender := &mockEmailSender{}
	w := &Worker{emailSender: sender}

	link := "http://localhost:5173/dashboard/proposals/xyz999"
	payload, _ := json.Marshal(map[string]string{
		"owner_email":    "owner@example.com",
		"proposal_title": "Proposal X",
		"client_name":    "Client Y",
		"proposal_link":  link,
		"country":        "",
	})

	if err := w.handleEmailOpenNotify(context.Background(), payload); err != nil {
		t.Fatalf("handleEmailOpenNotify error: %v", err)
	}

	if !strings.Contains(sender.calls[0].html, link) {
		t.Errorf("email body must contain proposal link %q", link)
	}
}

// TestWorker_EmailOpenNotify_ContainsCountry verifies that country is rendered
// in the email body when present.
func TestWorker_EmailOpenNotify_ContainsCountry(t *testing.T) {
	sender := &mockEmailSender{}
	w := &Worker{emailSender: sender}

	payload, _ := json.Marshal(map[string]string{
		"owner_email":    "owner@example.com",
		"proposal_title": "Proposal",
		"client_name":    "Client",
		"proposal_link":  "http://localhost/p/abc",
		"country":        "IT",
	})

	if err := w.handleEmailOpenNotify(context.Background(), payload); err != nil {
		t.Fatalf("handleEmailOpenNotify error: %v", err)
	}

	if !strings.Contains(sender.calls[0].html, "IT") {
		t.Error("email body must contain the country code when country is set")
	}
}

// TestWorker_EmailOpenNotify_NoCountryLine verifies that the location line is
// omitted from the body when country is empty.
func TestWorker_EmailOpenNotify_NoCountryLineWhenEmpty(t *testing.T) {
	sender := &mockEmailSender{}
	w := &Worker{emailSender: sender}

	payload, _ := json.Marshal(map[string]string{
		"owner_email":    "owner@example.com",
		"proposal_title": "Proposal",
		"client_name":    "Client",
		"proposal_link":  "http://localhost/p/abc",
		"country":        "",
	})

	if err := w.handleEmailOpenNotify(context.Background(), payload); err != nil {
		t.Fatalf("handleEmailOpenNotify error: %v", err)
	}

	if strings.Contains(sender.calls[0].html, "Location:") {
		t.Error("email body must not contain 'Location:' when country is empty")
	}
}

// TestWorker_EmailOpenNotify_InvalidPayload verifies that a malformed payload
// returns an error rather than panicking.
func TestWorker_EmailOpenNotify_InvalidPayload(t *testing.T) {
	w := &Worker{emailSender: &mockEmailSender{}}
	err := w.handleEmailOpenNotify(context.Background(), []byte("{bad json"))
	if err == nil {
		t.Error("expected error for invalid JSON payload, got nil")
	}
}

// TestWorker_EmailOpenNotify_UsesProposalTitleWhenNoClientName verifies the
// fallback subject when client_name equals the proposal title (no separate client).
func TestWorker_EmailOpenNotify_FallsBackToProposalTitle(t *testing.T) {
	sender := &mockEmailSender{}
	w := &Worker{emailSender: sender}

	payload, _ := json.Marshal(map[string]string{
		"owner_email":    "owner@example.com",
		"proposal_title": "Web Redesign 2026",
		"client_name":    "Web Redesign 2026", // same as title → fallback case
		"proposal_link":  "http://localhost/p/abc",
		"country":        "",
	})

	if err := w.handleEmailOpenNotify(context.Background(), payload); err != nil {
		t.Fatalf("handleEmailOpenNotify error: %v", err)
	}

	if !strings.Contains(sender.calls[0].subject, "Web Redesign 2026") {
		t.Errorf("subject must contain the display name, got %q", sender.calls[0].subject)
	}
}

// ─── Email handler isolation tests ───────────────────────────────────────────

// TestWorker_ApprovedNotify_SendsToOwner verifies that handleEmailApprovedNotify
// sends a notification to the owner with the correct subject.
func TestWorker_ApprovedNotify_SendsToOwner(t *testing.T) {
	sender := &mockEmailSender{}
	w := &Worker{emailSender: sender}

	payload, _ := json.Marshal(map[string]string{
		"owner_email":    "owner@example.com",
		"proposal_title": "SEO Package",
		"client_email":   "client@example.com",
		"approved_at":    "2026-03-31T10:00:00Z",
	})

	if err := w.handleEmailApprovedNotify(context.Background(), payload); err != nil {
		t.Fatalf("handleEmailApprovedNotify error: %v", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 email, got %d", len(sender.calls))
	}
	call := sender.calls[0]
	if call.to != "owner@example.com" {
		t.Errorf("expected to=owner@example.com, got %q", call.to)
	}
	if !strings.Contains(call.subject, "SEO Package") {
		t.Errorf("subject must contain proposal title, got %q", call.subject)
	}
	if !strings.Contains(call.html, "client@example.com") {
		t.Error("owner email body must include client email")
	}
}

// TestWorker_ClientApproved_SendsToClient verifies that handleEmailClientApproved
// sends a confirmation to the client with the agency name in the subject.
func TestWorker_ClientApproved_SendsToClient(t *testing.T) {
	sender := &mockEmailSender{}
	w := &Worker{emailSender: sender}

	payload, _ := json.Marshal(map[string]string{
		"client_email":   "client@example.com",
		"agency_name":    "Best Agency",
		"proposal_title": "SEO Package",
		"approved_at":    "2026-03-31T10:00:00Z",
	})

	if err := w.handleEmailClientApproved(context.Background(), payload); err != nil {
		t.Fatalf("handleEmailClientApproved error: %v", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 email, got %d", len(sender.calls))
	}
	call := sender.calls[0]
	if call.to != "client@example.com" {
		t.Errorf("expected to=client@example.com, got %q", call.to)
	}
	if !strings.Contains(call.subject, "Best Agency") {
		t.Errorf("subject must contain agency name, got %q", call.subject)
	}
}
