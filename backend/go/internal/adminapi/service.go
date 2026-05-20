// Package adminapi — trading-core API sub-component handlers backed by PostgreSQL.
package adminapi

import (
	"context"
	"fmt"

	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/db/pg"
	"go.uber.org/zap"
)

// Service holds all RPC service implementations for trading-core API layer.
type Service struct {
	pool        *pg.Pool
	log         *zap.Logger
	mt4Gateway  config.GatewayConfig
	mt5Gateway  config.GatewayConfig
	acctConn    AccountConnector
}

// AccountInfo holds data needed for persistent connection.
type AccountInfo struct {
	ID       string
	Login    string
	Password string
	Server   string
	Platform string
}

// AccountConnector is the interface for account connection management.
type AccountConnector interface {
	Connect(ctx context.Context, info AccountInfo)
	Disconnect(accountID string)
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

// WithAccountConnector sets the account connection manager for persistent MT links.
func (s *Service) WithAccountConnector(ac AccountConnector) *Service {
	s.acctConn = ac
	return s
}

// effectiveTenantID returns reqTenantID if non-empty, otherwise falls back to
// the context tenant ID. Returns empty string when neither is available.
func effectiveTenantID(ctx context.Context, reqTenantID string) string {
	if reqTenantID != "" {
		return reqTenantID
	}
	return auth.TenantFromContext(ctx)
}

// setRLS sets the tenant_id session variable for RLS, extracted from context.
// Returns an error when no tenant is available.
func (s *Service) setRLS(ctx context.Context) error {
	tenantID := auth.TenantFromContext(ctx)
	if tenantID == "" {
		return fmt.Errorf("no tenant in context")
	}
	return s.pool.SetTenant(ctx, tenantID)
}