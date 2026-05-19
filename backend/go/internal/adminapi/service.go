// Package adminapi — trading-core API sub-component handlers backed by PostgreSQL.
package adminapi

import (
	"context"

	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/common/db/pg"
)

// Service holds all RPC service implementations for trading-core API layer.
type Service struct {
	pool *pg.Pool
}

// NewService creates a trading-core API service backed by a PG connection pool.
func NewService(pool *pg.Pool) *Service {
	return &Service{pool: pool}
}

// defaultTenantID is used when no tenant is available from context.
const defaultTenantID = "00000000-0000-0000-0000-000000000000"

// effectiveTenantID returns reqTenantID if non-empty, otherwise falls back to
// the context tenant ID, and finally to the default tenant.
func effectiveTenantID(ctx context.Context, reqTenantID string) string {
	if reqTenantID != "" {
		return reqTenantID
	}
	if tid := auth.TenantFromContext(ctx); tid != "" {
		return tid
	}
	return defaultTenantID
}

// setRLS sets the tenant_id session variable for RLS, extracted from context.
func (s *Service) setRLS(ctx context.Context) error {
	tenantID := auth.TenantFromContext(ctx)
	if tenantID == "" {
		tenantID = defaultTenantID
	}
	return s.pool.SetTenant(ctx, tenantID)
}
