// Package adminapi — JWT authentication + RLS middleware for Connect RPC handlers.
package adminapi

import (
	"context"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/jackc/pgx/v5"
)

type ctxKey string

const txCtxKey ctxKey = "adminapi_tx"

// TxFromContext returns the pgx.Tx stored in context by RLSInterceptor, or nil.
func TxFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(txCtxKey).(pgx.Tx)
	return tx
}

// AuthMiddleware returns an HTTP middleware that validates JWT Bearer tokens,
// checks the token blacklist, and injects tenant/user/role/email into the request context.
// Requests without a valid token pass through unauthenticated; individual handlers
// are responsible for rejecting unauthenticated access via RequireTenant.
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

// RLSInterceptor is a Connect interceptor that wraps each request in a pgx.Tx
// with SET LOCAL app.tenant_id, eliminating the need for manual setRLS() calls.
func RLSInterceptor(pool *pg.Pool) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			tenantID := auth.TenantFromContext(ctx)
			if tenantID == "" || pool == nil {
				return next(ctx, req)
			}

			tx, err := pool.BeginTx(ctx, tenantID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			defer func() {
				_ = tx.Rollback(ctx) // no-op if already committed
			}()

			ctx = context.WithValue(ctx, txCtxKey, tx)
			resp, err := next(ctx, req)
			if err != nil {
				return resp, err
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, connect.NewError(connect.CodeInternal, commitErr)
			}
			return resp, nil
		}
	}
}

// RequireTenant is a helper that rejects requests without a tenant in context.
// Use in handlers that must have authentication.
func RequireTenant(ctx context.Context) error {
	if auth.TenantFromContext(ctx) == "" {
		return ErrSessionExpired
	}
	return nil
}

func extractBearerToken(r *http.Request) string {
	authHdr := r.Header.Get("Authorization")
	if authHdr == "" {
		return ""
	}
	if !strings.HasPrefix(authHdr, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(authHdr, "Bearer ")
}
