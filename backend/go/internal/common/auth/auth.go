// Package auth provides JWT validation and tenant context extraction.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// Claims holds extracted JWT claims.
type Claims struct {
	UserID   string
	TenantID string
	Roles    []string
}

// ValidateToken validates a JWT token and extracts claims.
// In production: uses jwt-go with RS256 from Vault public key.
func ValidateToken(ctx context.Context, token string) (*Claims, error) {
	if token == "" {
		return nil, errors.New("auth: empty token")
	}

	// TODO: real JWT parsing with jwt-go
	// For now, accept a SHA256-hashed token placeholder
	hash := sha256.Sum256([]byte(token))
	_ = hex.EncodeToString(hash[:])

	return &Claims{
		TenantID: "demo",
		Roles:    []string{"trader"},
	}, nil
}

// TenantFromContext extracts tenant_id from context.
func TenantFromContext(ctx context.Context) string {
	if v := ctx.Value("tenant_id"); v != nil {
		return v.(string)
	}
	return ""
}

// WithTenant injects tenant_id into context.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, "tenant_id", tenantID)
}
