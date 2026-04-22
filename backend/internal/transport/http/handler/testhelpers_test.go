package handler_test

import (
	"context"
	"net/http"

	"proply/internal/config"
	"proply/internal/service"
	"proply/internal/transport/http/handler"
	"proply/internal/transport/http/middleware"
	pkgjwt "proply/pkg/jwt"
)

// withFakeClaims injects a minimal JWT Claims struct into the request context,
// simulating a successfully authenticated request without a real JWT token.
func withFakeClaims(r *http.Request) *http.Request {
	claims := &pkgjwt.Claims{
		UserID:        "test-user-id",
		Email:         "test@example.com",
		Plan:          "free",
		EmailVerified: false,
	}
	ctx := context.WithValue(r.Context(), middleware.ClaimsKey, claims)
	return r.WithContext(ctx)
}

// withVerifiedFakeClaims is the same as withFakeClaims but with EmailVerified=true.
// Use this for tests that must pass the email-verification gate.
func withVerifiedFakeClaims(r *http.Request) *http.Request {
	claims := &pkgjwt.Claims{
		UserID:        "test-user-id",
		Email:         "test@example.com",
		Plan:          "free",
		EmailVerified: true,
	}
	ctx := context.WithValue(r.Context(), middleware.ClaimsKey, claims)
	return r.WithContext(ctx)
}

// withVerifiedProFakeClaims injects claims for a verified Pro-plan user.
func withVerifiedProFakeClaims(r *http.Request) *http.Request {
	claims := &pkgjwt.Claims{
		UserID:        "test-pro-user-id",
		Email:         "pro@example.com",
		Plan:          "pro",
		EmailVerified: true,
	}
	ctx := context.WithValue(r.Context(), middleware.ClaimsKey, claims)
	return r.WithContext(ctx)
}

// newUnconfiguredStorageHandler builds an UploadHandler backed by a
// StorageService that has no S3 credentials configured (presignClient == nil).
// This allows testing the STORAGE_NOT_CONFIGURED response path.
func newUnconfiguredStorageHandler() *handler.UploadHandler {
	cfg := &config.Config{} // all S3 fields empty → presignClient remains nil
	svc := service.NewStorageService(cfg)
	return handler.NewUploadHandler(svc)
}
