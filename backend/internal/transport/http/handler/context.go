package handler

import (
	"net/http"

	"proply/internal/transport/http/middleware"
	pkgjwt "proply/pkg/jwt"
)

// claimsFromCtx is a convenience wrapper to extract JWT claims from the request context.
func claimsFromCtx(r *http.Request) *pkgjwt.Claims {
	return middleware.ClaimsFromCtx(r.Context())
}
