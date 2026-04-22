package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port string
	Env  string // "development" | "production"

	// Database
	DatabaseURL string

	// JWT
	JWTSecret          string
	JWTAccessExpiryMin int // minutes
	JWTRefreshExpiryDay int // days

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	OAuthStateSecret   string // HMAC key for state parameter validation

	// Email
	ResendAPIKey  string
	EmailFromAddr string
	EmailFromName string
	// SMTP — used when ResendAPIKey is empty (e.g. Mailpit on the test stand)
	SMTPHost string
	SMTPPort int

	// Stripe
	StripeSecretKey    string
	StripeWebhookSecret string
	StripePriceProID   string
	StripePriceTeamID  string

	// Hetzner Object Storage (S3-compatible)
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3Region    string

	// App
	AppURL       string // e.g. https://proply.io
	InternalURL  string // Go API internal URL (called by SvelteKit)
}

// Load reads configuration from environment variables.
// Returns an error if any required variable is missing.
func Load() (*Config, error) {
	cfg := &Config{
		Port:                getEnv("PORT", "8080"),
		Env:                 getEnv("ENV", "development"),
		DatabaseURL:         requireEnv("DATABASE_URL"),
		JWTSecret:           requireEnv("JWT_SECRET"),
		JWTAccessExpiryMin:  getEnvInt("JWT_ACCESS_EXPIRY_MIN", 15),
		JWTRefreshExpiryDay: getEnvInt("JWT_REFRESH_EXPIRY_DAYS", 30),
		GoogleClientID:      getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:  getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:   getEnv("GOOGLE_REDIRECT_URL", ""),
		OAuthStateSecret:    getEnv("OAUTH_STATE_SECRET", ""),
		ResendAPIKey:        getEnv("RESEND_API_KEY", ""),
		EmailFromAddr:       getEnv("EMAIL_FROM_ADDR", "noreply@proply.io"),
		EmailFromName:       getEnv("EMAIL_FROM_NAME", "Proply"),
		SMTPHost:            getEnv("SMTP_HOST", ""),
		SMTPPort:            getEnvInt("SMTP_PORT", 1025),
		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripePriceProID:    getEnv("STRIPE_PRICE_PRO_ID", ""),
		StripePriceTeamID:   getEnv("STRIPE_PRICE_TEAM_ID", ""),
		S3Endpoint:          getEnv("S3_ENDPOINT", ""),
		S3AccessKey:         getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:         getEnv("S3_SECRET_KEY", ""),
		S3Bucket:            getEnv("S3_BUCKET", "proply-media"),
		S3Region:            getEnv("S3_REGION", "eu-central"),
		AppURL:              getEnv("APP_URL", "http://localhost:5173"),
		InternalURL:         getEnv("INTERNAL_URL", "http://localhost:8080"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func requireEnv(key string) string {
	return os.Getenv(key)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
