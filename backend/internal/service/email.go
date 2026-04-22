package service

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

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

// SmtpEmailSender sends emails via a plain SMTP relay (e.g. Mailpit on the test stand).
// It uses AUTH PLAIN when credentials are provided; otherwise it connects anonymously —
// which is the default for Mailpit (no auth required).
type SmtpEmailSender struct {
	addr      string // host:port
	fromAddr  string
	fromName  string
}

// NewSmtpEmailSender creates a new SmtpEmailSender.
// host and port are taken from SMTP_HOST / SMTP_PORT environment variables.
func NewSmtpEmailSender(host string, port int, fromAddr, fromName string) *SmtpEmailSender {
	return &SmtpEmailSender{
		addr:     fmt.Sprintf("%s:%d", host, port),
		fromAddr: fromAddr,
		fromName: fromName,
	}
}

// Send sends an HTML email via SMTP.
func (s *SmtpEmailSender) Send(_ context.Context, to, subject, html string) error {
	from := fmt.Sprintf("%s <%s>", s.fromName, s.fromAddr)
	msg := strings.Join([]string{
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"",
		html,
	}, "\r\n")

	// SendMail with nil auth works for unauthenticated relays like Mailpit.
	if err := smtp.SendMail(s.addr, nil, s.fromAddr, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp: send email to %s: %w", to, err)
	}
	return nil
}
