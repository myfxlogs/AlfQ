// Package adminapi — JWT authentication middleware for Connect RPC handlers.
package adminapi

import (
	"net/http"
	"strings"

	"github.com/alfq/backend/go/internal/common/auth"
)

// AuthMiddleware returns an HTTP middleware that validates JWT Bearer tokens,
// checks the token blacklist, and injects tenant/user/role/email into the request context.
// Requests without a valid token pass through unauthenticated; individual handlers
// are responsible for rejecting unauthenticated access via setRLS.
func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := auth.Verify(token, map[string]auth.Ed25519PublicKey{
			h.kp.Kid: h.kp.PublicKey,
		})
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		if claims.IsExpired() {
			next.ServeHTTP(w, r)
			return
		}
		if h.IsTokenBlacklisted(r.Context(), token) {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		ctx = auth.WithTenant(ctx, claims.TenantID)
		ctx = auth.WithUser(ctx, claims.Sub)
		ctx = auth.WithRoles(ctx, claims.Roles)
		ctx = auth.WithEmail(ctx, claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}
