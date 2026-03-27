package service

import (
	"context"
	"fmt"

	resend "github.com/resend/resend-go/v2"
)

// ResendEmailSender sends emails via the Resend API.
type ResendEmailSender struct {
	client    *resend.Client
	fromAddr  string
	fromName  string
}

// NewResendEmailSender creates a new ResendEmailSender.
func NewResendEmailSender(apiKey, fromAddr, fromName string) *ResendEmailSender {
	return &ResendEmailSender{
		client:   resend.NewClient(apiKey),
		fromAddr: fromAddr,
		fromName: fromName,
	}
}

// Send sends an HTML email via Resend.
func (s *ResendEmailSender) Send(ctx context.Context, to, subject, html string) error {
	params := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromAddr),
		To:      []string{to},
		Subject: subject,
		Html:    html,
	}

	_, err := s.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("resend: send email to %s: %w", to, err)
	}
	return nil
}
