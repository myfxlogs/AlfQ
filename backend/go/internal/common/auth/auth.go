// Package auth provides JWT validation, tenant/user context extraction, and type aliases.
package auth

import (
	"context"
	"crypto/ed25519"
)

// Ed25519PublicKey is an alias for crypto/ed25519.PublicKey.
type Ed25519PublicKey = ed25519.PublicKey

// contextKey is the unexported type for context keys.
type contextKey string

const (
	ctxTenantID contextKey = "tenant_id"
	ctxUserID   contextKey = "user_id"
	ctxRoles    contextKey = "roles"
	ctxEmail    contextKey = "email"
)

// ValidateToken parses and validates a JWT token, returning the claims.
// This is a convenience wrapper around Verify.
func ValidateToken(ctx context.Context, token string, keys map[string]Ed25519PublicKey) (*Claims, error) {
	return Verify(token, keys)
}

// TenantFromContext extracts tenant_id from context.
func TenantFromContext(ctx context.Context) string {
	if v := ctx.Value(ctxTenantID); v != nil {
		return v.(string)
	}
	return ""
}

// UserFromContext extracts user_id from context.
func UserFromContext(ctx context.Context) string {
	if v := ctx.Value(ctxUserID); v != nil {
		return v.(string)
	}
	return ""
}

// RolesFromContext extracts roles from context.
func RolesFromContext(ctx context.Context) []string {
	if v := ctx.Value(ctxRoles); v != nil {
		return v.([]string)
	}
	return nil
}

// EmailFromContext extracts email from context.
func EmailFromContext(ctx context.Context) string {
	if v := ctx.Value(ctxEmail); v != nil {
		return v.(string)
	}
	return ""
}

// WithTenant injects tenant_id into context.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ctxTenantID, tenantID)
}

// WithUser injects user_id into context.
func WithUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxUserID, userID)
}

// WithRoles injects roles into context.
func WithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, ctxRoles, roles)
}

// WithEmail injects email into context.
func WithEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, ctxEmail, email)
}
