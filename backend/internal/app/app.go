package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"proply/internal/config"
	"proply/internal/repository"
	"proply/internal/service"
	"proply/internal/transport/http/handler"
	"proply/internal/transport/http/middleware"
	pkgjwt "proply/pkg/jwt"
)

// App wires together all dependencies and runs the HTTP server.
type App struct {
	cfg    *config.Config
	server *http.Server
}

// New creates and configures a new App instance.
func New(cfg *config.Config) (*App, error) {
	// Configure structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Database
	ctx := context.Background()
	db, err := repository.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("app: database: %w", err)
	}

	// JWT manager
	jwtMgr := pkgjwt.NewManager(cfg.JWTSecret, cfg.JWTAccessExpiryMin)

	// Services
	authSvc := service.NewAuthService(db, cfg.AppURL, cfg.EmailFromAddr, cfg.EmailFromName)
	proposalSvc := service.NewProposalService(db)
	trackingSvc := service.NewTrackingService(db, cfg.AppURL)
	storageSvc := service.NewStorageService(cfg)
	billingSvc := service.NewBillingService(db, cfg)
	accountSvc := service.NewAccountService(db, storageSvc, billingSvc)

	// Email sender — priority: Resend (production) → SMTP (test stand / Mailpit) → noop
	var emailSender service.EmailSender
	switch {
	case cfg.ResendAPIKey != "":
		emailSender = service.NewResendEmailSender(cfg.ResendAPIKey, cfg.EmailFromAddr, cfg.EmailFromName)
	case cfg.SMTPHost != "":
		slog.Info("app: using SMTP email sender", "host", cfg.SMTPHost, "port", cfg.SMTPPort)
		emailSender = service.NewSmtpEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.EmailFromAddr, cfg.EmailFromName)
	default:
		slog.Warn("app: no email sender configured, email sending is disabled")
		emailSender = &noopEmailSender{}
	}

	// Background worker
	worker := service.NewWorker(db, emailSender, accountSvc)

	// Handlers
	authH := handler.NewAuthHandler(authSvc, jwtMgr, cfg)
	proposalH := handler.NewProposalHandler(proposalSvc)
	publicH := handler.NewPublicHandler(proposalSvc)
	trackingH := handler.NewTrackingHandler(trackingSvc)
	billingH := handler.NewBillingHandler(billingSvc)
	accountH := handler.NewAccountHandler(accountSvc)
	uploadH := handler.NewUploadHandler(storageSvc)

	// Router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.AppURL},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Proposal-Password"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Auth routes
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", authH.Register)
		r.Post("/login", authH.Login)
		r.Post("/refresh", authH.Refresh)
		r.Post("/logout", authH.Logout)
		r.With(middleware.Auth(jwtMgr)).Get("/me", authH.Me)
		r.With(middleware.Auth(jwtMgr)).Post("/resend-verification", authH.ResendVerification)
		// Email verification (token via query param, redirects to SvelteKit)
		r.Get("/verify-email", authH.VerifyEmail)
		// Magic link (passwordless login)
		r.Post("/magic-link", authH.MagicLink)
		r.Get("/magic-link/verify", authH.MagicLinkVerify)
		// Google OAuth
		r.Get("/google", authH.GoogleRedirect)
		r.Get("/google/callback", authH.GoogleCallback)
	})

	// Template catalog (public — no auth required)
	r.Get("/api/v1/templates", proposalH.ListTemplates)

	// Authenticated proposal routes
	r.Route("/api/v1/proposals", func(r chi.Router) {
		r.Use(middleware.Auth(jwtMgr))
		r.Get("/", proposalH.List)
		r.Post("/", proposalH.Create)
		r.Get("/{id}", proposalH.Get)
		r.Patch("/{id}", proposalH.Update)
		r.Delete("/{id}", proposalH.Delete)
		r.Post("/{id}/publish", proposalH.Publish)
		r.Patch("/{id}/status", proposalH.UpdateStatus)
		r.Post("/{id}/revoke", proposalH.Revoke)
		r.Post("/{id}/duplicate", proposalH.Duplicate)
		r.Get("/{id}/analytics", proposalH.GetAnalytics)
	})

	// Public endpoints (no auth)
	r.Route("/api/v1/public", func(r chi.Router) {
		r.Get("/proposals/{slug}", publicH.GetProposal)
		r.Post("/proposals/{slug}/approve", publicH.Approve)
	})

	// Internal tracking (SvelteKit → Go, server-to-server)
	r.Route("/api/v1/internal", func(r chi.Router) {
		// TODO: add InternalOnly middleware with shared secret
		r.Post("/track/open", trackingH.TrackOpen)
	})

	// Client-side tracking (first-party, no auth needed)
	r.Post("/api/v1/track/block-time", trackingH.TrackBlocks)

	// Billing webhooks
	r.Post("/api/v1/webhooks/stripe", billingH.StripeWebhook)

	// Billing management (authenticated)
	r.Route("/api/v1/billing", func(r chi.Router) {
		r.Use(middleware.Auth(jwtMgr))
		r.Post("/checkout", billingH.CreateCheckout)
		r.Post("/portal", billingH.CreatePortal)
	})

	// Account settings (authenticated)
	r.Route("/api/v1/account", func(r chi.Router) {
		r.Use(middleware.Auth(jwtMgr))
		r.Patch("/", accountH.UpdateProfile)
		r.Patch("/branding", accountH.UpdateBranding)
		r.Patch("/retention", accountH.UpdateRetention)
		r.Get("/export", accountH.ExportData)
		r.Delete("/", accountH.DeleteAccount)
	})

	// Media upload — presigned URL generation (authenticated)
	r.With(middleware.Auth(jwtMgr)).Post("/api/v1/upload/presign", uploadH.Presign)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	app := &App{cfg: cfg, server: server}

	// Start background worker
	go worker.Run(ctx)

	return app, nil
}

// Run starts the HTTP server.
func (a *App) Run() error {
	slog.Info("server starting", "port", a.cfg.Port, "env", a.cfg.Env)
	return a.server.ListenAndServe()
}

// noopEmailSender discards emails (used when Resend API key is not configured).
type noopEmailSender struct{}

func (n *noopEmailSender) Send(_ context.Context, to, subject, _ string) error {
	slog.Warn("email not sent (noop sender)", "to", to, "subject", subject)
	return nil
}
