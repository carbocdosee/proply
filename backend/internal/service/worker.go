package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Worker polls the job_queue table and processes pending background jobs.
// Runs as a goroutine in the same binary as the HTTP server.
type Worker struct {
	db          *pgxpool.Pool
	emailSender EmailSender
}

// EmailSender defines the interface for sending emails.
type EmailSender interface {
	Send(ctx context.Context, to, subject, html string) error
}

// NewWorker creates a new Worker.
func NewWorker(db *pgxpool.Pool, emailSender EmailSender) *Worker {
	return &Worker{db: db, emailSender: emailSender}
}

// Run starts the polling loop. Polls every 5 seconds.
// Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	slog.Info("worker: started, polling every 5s")

	for {
		select {
		case <-ctx.Done():
			slog.Info("worker: stopped")
			return
		case <-ticker.C:
			if err := w.processJobs(ctx); err != nil {
				slog.Error("worker: process jobs error", "error", err)
			}
		}
	}
}

// processJobs fetches and processes up to 10 pending jobs.
func (w *Worker) processJobs(ctx context.Context) error {
	rows, err := w.db.Query(ctx, `
		UPDATE job_queue SET status='processing', attempts=attempts+1
		WHERE id IN (
			SELECT id FROM job_queue
			WHERE status='pending' AND scheduled_at <= NOW()
			ORDER BY scheduled_at ASC
			LIMIT 10
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, job_type, payload
	`)
	if err != nil {
		return fmt.Errorf("worker: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, jobType string
		var payload json.RawMessage
		if err := rows.Scan(&id, &jobType, &payload); err != nil {
			continue
		}
		if err := w.processJob(ctx, id, jobType, payload); err != nil {
			slog.Error("worker: job failed", "id", id, "type", jobType, "error", err)
			w.markFailed(ctx, id, err.Error())
		}
	}
	return nil
}

// processJob dispatches a single job by type.
func (w *Worker) processJob(ctx context.Context, id, jobType string, payload json.RawMessage) error {
	defer func() {
		_, _ = w.db.Exec(ctx,
			`UPDATE job_queue SET status='done', processed_at=NOW() WHERE id=$1`, id)
	}()

	switch jobType {
	case "email_open_notify":
		return w.handleEmailOpenNotify(ctx, payload)
	case "email_approved_notify":
		return w.handleEmailApprovedNotify(ctx, payload)
	case "email_client_approved":
		return w.handleEmailClientApproved(ctx, payload)
	case "email_verification":
		return w.handleEmailVerification(ctx, payload)
	case "email_magic_link":
		return w.handleEmailMagicLink(ctx, payload)
	case "gdpr_hard_delete":
		return w.handleGDPRHardDelete(ctx, payload)
	default:
		slog.Warn("worker: unknown job type", "type", jobType)
		return nil
	}
}

func (w *Worker) handleEmailOpenNotify(ctx context.Context, payload json.RawMessage) error {
	var p struct {
		OwnerEmail    string `json:"owner_email"`
		ProposalTitle string `json:"proposal_title"`
		Country       string `json:"country"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("email_open_notify: unmarshal: %w", err)
	}

	subject := fmt.Sprintf("Клиент открыл ваш proposal: %s", p.ProposalTitle)
	body := fmt.Sprintf(`<p>Ваш proposal <strong>%s</strong> был открыт клиентом.</p>`, p.ProposalTitle)
	if p.Country != "" {
		body += fmt.Sprintf(`<p>Страна: %s</p>`, p.Country)
	}

	return w.emailSender.Send(ctx, p.OwnerEmail, subject, body)
}

func (w *Worker) handleEmailApprovedNotify(ctx context.Context, payload json.RawMessage) error {
	var p struct {
		OwnerEmail    string `json:"owner_email"`
		ProposalTitle string `json:"proposal_title"`
		ClientEmail   string `json:"client_email"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("email_approved_notify: unmarshal: %w", err)
	}

	subject := fmt.Sprintf("Proposal согласован: %s", p.ProposalTitle)
	body := fmt.Sprintf(`<p>Клиент <strong>%s</strong> согласовал proposal <strong>%s</strong>.</p>`,
		p.ClientEmail, p.ProposalTitle)

	return w.emailSender.Send(ctx, p.OwnerEmail, subject, body)
}

func (w *Worker) handleEmailClientApproved(ctx context.Context, payload json.RawMessage) error {
	var p struct {
		ClientEmail   string `json:"client_email"`
		AgencyName    string `json:"agency_name"`
		ProposalTitle string `json:"proposal_title"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("email_client_approved: unmarshal: %w", err)
	}

	subject := fmt.Sprintf("Вы согласовали proposal от %s", p.AgencyName)
	body := fmt.Sprintf(`<p>Вы согласовали коммерческое предложение <strong>%s</strong> от <strong>%s</strong>. Мы скоро свяжемся с вами.</p>`,
		p.ProposalTitle, p.AgencyName)

	return w.emailSender.Send(ctx, p.ClientEmail, subject, body)
}

func (w *Worker) handleEmailVerification(ctx context.Context, payload json.RawMessage) error {
	var p struct {
		To    string `json:"to"`
		Link  string `json:"link"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("email_verification: unmarshal: %w", err)
	}
	subject := "Подтвердите ваш email — Proply"
	body := fmt.Sprintf(`
		<p>Добро пожаловать в Proply!</p>
		<p>Нажмите кнопку ниже, чтобы подтвердить email-адрес <strong>%s</strong>:</p>
		<p><a href="%s" style="display:inline-block;padding:12px 24px;background:#6366f1;color:#fff;border-radius:8px;text-decoration:none;font-weight:600">Подтвердить email</a></p>
		<p>Ссылка действительна 24 часа.</p>
		<p style="color:#9ca3af;font-size:12px">Если вы не регистрировались в Proply, просто проигнорируйте это письмо.</p>
	`, p.Email, p.Link)
	return w.emailSender.Send(ctx, p.To, subject, body)
}

func (w *Worker) handleEmailMagicLink(ctx context.Context, payload json.RawMessage) error {
	var p struct {
		To    string `json:"to"`
		Link  string `json:"link"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("email_magic_link: unmarshal: %w", err)
	}
	subject := "Ваша ссылка для входа — Proply"
	body := fmt.Sprintf(`
		<p>Кто-то запросил вход в Proply для <strong>%s</strong>.</p>
		<p>Нажмите кнопку ниже, чтобы войти (ссылка действительна 15 минут):</p>
		<p><a href="%s" style="display:inline-block;padding:12px 24px;background:#6366f1;color:#fff;border-radius:8px;text-decoration:none;font-weight:600">Войти в Proply</a></p>
		<p style="color:#9ca3af;font-size:12px">Если вы не запрашивали эту ссылку, просто проигнорируйте письмо.</p>
	`, p.Email, p.Link)
	return w.emailSender.Send(ctx, p.To, subject, body)
}

func (w *Worker) handleGDPRHardDelete(ctx context.Context, payload json.RawMessage) error {
	var p struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("gdpr_hard_delete: unmarshal: %w", err)
	}

	_, err := w.db.Exec(ctx, `DELETE FROM users WHERE id=$1 AND deleted_at IS NOT NULL`, p.UserID)
	return err
}

func (w *Worker) markFailed(ctx context.Context, id, errMsg string) {
	_, _ = w.db.Exec(ctx, `
		UPDATE job_queue SET
			status = CASE WHEN attempts >= max_attempts THEN 'failed' ELSE 'pending' END,
			error = $1,
			scheduled_at = NOW() + INTERVAL '1 minute' * attempts
		WHERE id = $2
	`, errMsg, id)
}
