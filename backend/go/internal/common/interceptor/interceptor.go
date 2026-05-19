// Package interceptor provides Connect/gRPC interceptor chain with JWT auth and RBAC.
package interceptor

import (
	"context"
	"strings"

	"github.com/alfq/backend/go/internal/common/auth"

	"connectrpc.com/connect"
)

// KeySet maps kid to Ed25519 public key for JWT verification.
type KeySet map[string]auth.Ed25519PublicKey

// TokenBlacklist checks whether a token has been revoked.
type TokenBlacklist interface {
	IsTokenBlacklisted(ctx context.Context, token string) bool
}

// NewAuthInterceptor returns a Connect unary interceptor that:
//  1. Extracts JWT from Authorization header
//  2. Verifies the JWT signature and expiry
//  3. Checks the token blacklist
//  4. Injects tenant_id, user_id, roles into context
//
// If the request path is a health check or unauthenticated endpoint (login), it passes through.
func NewAuthInterceptor(keys KeySet, bl TokenBlacklist) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// Skip auth for health checks and login
			path := req.Spec().Procedure
			if isUnauthenticated(path) {
				return next(ctx, req)
			}

			// Extract Bearer token
			authHeader := req.Header().Get("Authorization")
			if authHeader == "" {
				// Also try X-Tenant-ID for backward compat during migration
				tenantID := req.Header().Get("X-Tenant-ID")
				if tenantID != "" {
					ctx = auth.WithTenant(ctx, tenantID)
				}
				return next(ctx, req)
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmtError("missing Bearer token"))
			}

			// Verify JWT
			claims, err := auth.Verify(token, map[string]auth.Ed25519PublicKey(keys))
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmtError("invalid token: "+err.Error()))
			}

			// Check expiry
			if claims.IsExpired() {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmtError("token expired"))
			}

			// Check blacklist
			if bl.IsTokenBlacklisted(ctx, token) {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmtError("token revoked"))
			}

			// Inject claims into context
			ctx = auth.WithTenant(ctx, claims.TenantID)
			ctx = auth.WithUser(ctx, claims.Sub)
			ctx = auth.WithRoles(ctx, claims.Roles)
			ctx = auth.WithEmail(ctx, claims.Email)

			// RBAC check: verify the caller has permission for this RPC
			if !hasPermission(claims.Roles, path) {
				return nil, connect.NewError(connect.CodePermissionDenied, fmtError("insufficient permissions"))
			}

			return next(ctx, req)
		})
	})
}

// isUnauthenticated returns true for endpoints that don't require auth.
func isUnauthenticated(procedure string) bool {
	return strings.Contains(procedure, "HealthService") ||
		strings.Contains(procedure, "AuthService/Login") ||
		strings.Contains(procedure, "AuthService/RefreshToken")
}

// hasPermission checks if any of the roles has access to the given RPC procedure.
// Roles are checked hierarchically: admin > risk_officer > trader > analyst
func hasPermission(roles []string, procedure string) bool {
	for _, role := range roles {
		switch role {
		case "admin":
			return true // admin can access everything
		case "risk_officer":
			if allowedRiskOfficer(procedure) {
				return true
			}
		case "trader":
			if allowedTrader(procedure) {
				return true
			}
		case "analyst":
			if allowedAnalyst(procedure) {
				return true
			}
		}
	}
	return false
}

func allowedAdmin(procedure string) bool  { return true }
func allowedTrader(procedure string) bool {
	return !strings.Contains(procedure, "TenantService") &&
		!strings.Contains(procedure, "UserService") &&
		!strings.Contains(procedure, "AuditService")
}
func allowedAnalyst(procedure string) bool {
	return strings.Contains(procedure, "FactorService") ||
		strings.Contains(procedure, "BacktestService") ||
		strings.Contains(procedure, "StrategyService/List") ||
		strings.Contains(procedure, "StrategyService/Get") ||
		strings.Contains(procedure, "SymbolService") ||
		strings.Contains(procedure, "MarketDataService")
}
func allowedRiskOfficer(procedure string) bool {
	return strings.Contains(procedure, "RiskService") ||
		strings.Contains(procedure, "AuditService") ||
		strings.Contains(procedure, "OrderService/List") ||
		strings.Contains(procedure, "PositionService")
}

func fmtError(msg string) error {
	return &stringError{msg: msg}
}

type stringError struct{ msg string }

func (e *stringError) Error() string { return e.msg }
