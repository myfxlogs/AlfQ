// Package adminapi — trading-core API sub-component handlers backed by PostgreSQL.
package adminapi

import (
	"context"

	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/db/pg"
	"go.uber.org/zap"
)

// Service holds all RPC service implementations for trading-core API layer.
type Service struct {
	pool       *pg.Pool
	log        *zap.Logger
	mt4Gateway config.GatewayConfig
	mt5Gateway config.GatewayConfig
}

// NewService creates a trading-core API service backed by a PG connection pool.
func NewService(pool *pg.Pool) *Service {
	return &Service{pool: pool, log: zap.NewNop()}
}

// WithGateways sets MT4/MT5 gateway configuration for online broker search.
func (s *Service) WithGateways(mt4, mt5 config.GatewayConfig) *Service {
	s.mt4Gateway = mt4
	s.mt5Gateway = mt5
	return s
}

// WithLog sets a logger (defaults to nop).
func (s *Service) WithLog(log *zap.Logger) *Service {
	s.log = log
	return s
}

// defaultTenantID is used when no tenant is available from context.
const defaultTenantID = "00000000-0000-0000-0000-000000000000"

// defaultBrokerID is the placeholder used by BindAccount when no broker exists.
const defaultBrokerID = "00000000-0000-0000-0000-000000000000"

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
