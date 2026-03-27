package middleware

import (
	"context"
	"net/http"
	"strings"

	pkgjwt "proply/pkg/jwt"
)

type contextKey string

const ClaimsKey contextKey = "claims"

// Auth is a JWT authentication middleware.
// It expects a Bearer token in the Authorization header.
// On success, it stores the parsed claims in the request context.
func Auth(jwtManager *pkgjwt.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				http.Error(w, `{"code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}

			claims, err := jwtManager.ParseAccess(parts[1])
			if err != nil {
				http.Error(w, `{"code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// InternalOnly restricts access to requests from SvelteKit (Railway internal network).
// Checks the X-Internal-Secret header against the configured secret.
func InternalOnly(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Internal-Secret") != secret {
				http.Error(w, `{"code":"FORBIDDEN"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClaimsFromCtx extracts JWT claims from the request context.
func ClaimsFromCtx(ctx context.Context) *pkgjwt.Claims {
	v, _ := ctx.Value(ClaimsKey).(*pkgjwt.Claims)
	return v
}
