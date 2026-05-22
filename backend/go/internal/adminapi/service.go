// Package adminapi — trading-core API sub-component handlers backed by PostgreSQL.
package adminapi

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/alfq/backend/go/internal/accountconn"
	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/crypto"
	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/alfq/backend/go/internal/oms/repo"
	"go.uber.org/zap"
)

// ErrSessionExpired is returned when no tenant is bound to the request context
// (no token, expired token, or signature mismatch e.g. after a key rotation).
// It carries a `connect.CodeUnauthenticated` so the client transport can route
// the user to the login screen instead of showing a 500.
var ErrSessionExpired = connect.NewError(
	connect.CodeUnauthenticated,
	errors.New("会话已过期或无效，请重新登录"),
)

// Service holds all RPC service implementations for trading-core API layer.
type Service struct {
	pool              *pg.Pool
	log               *zap.Logger
	mt4Gateway        config.GatewayConfig
	mt5Gateway        config.GatewayConfig
	acctConn          AccountConnector
	syncWorker        OrderSyncer
	historyRepo       HistoryOrderRepo
	publishSyncDoneFn func(accountID string)
	encCipher         *crypto.AESCipher // R10: AES-256-GCM for API keys
	symbolResolver    *SymbolResolver  // RS06: validates strategy symbols against broker
}

// OrderSyncer abstracts the order history sync worker.
type OrderSyncer interface {
	FullSync(ctx context.Context, accountID string) error
	GetSyncState(ctx context.Context, accountID string) (*accountconn.SyncState, error)
}

// HistoryOrderRepo abstracts the local order-history repository.
type HistoryOrderRepo interface {
	List(ctx context.Context, tenantID, accountID string, from, to time.Time) ([]*repo.HistoryOrder, error)
}

// AccountInfo holds data needed for persistent connection.
type AccountInfo struct {
	ID       string
	Login    string
	Password string
	Server   string
	Platform string
	BrokerID string
}

// AccountConnector is the interface for account connection management.
type AccountConnector interface {
	Connect(ctx context.Context, info AccountInfo)
	Disconnect(accountID string)
	// LatestPositions returns the most-recent positions for an account, or nil if
	// no live session is available. Implementations must be non-blocking.
	LatestPositions(accountID string) []*PositionInfo
	// RefreshPositions fetches fresh positions from the broker (via mthub).
	// Call before LatestPositions to get live current prices.
	RefreshPositions(ctx context.Context, accountID string)
	// WithLiveSession invokes fn with the live gateway gRPC connection and session ID.
	// Returns an error if no live session is available. Implementations must not
	// block on dialing — the live session is expected to already exist.
	WithLiveSession(accountID string, fn func(conn interface{}, sessionID, platform string) error) error
}

// PositionInfo is a unified position record exposed by AccountConnector.
type PositionInfo struct {
	Ticket       int64
	Symbol       string
	Type         string
	Lots         float64
	OpenPrice    float64
	Profit       float64
	Swap         float64
	Commission   float64
	OpenTimeMs   int64   // position open timestamp (UTC ms)
	CurrentPrice float64 // latest bid/ask
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

// WithEncCipher sets the AES-256-GCM cipher for API key encryption (R10).
func (s *Service) WithEncCipher(c *crypto.AESCipher) *Service {
	s.encCipher = c
	return s
}

// WithSymbolResolver enables strategy symbol validation (RS06).
func (s *Service) WithSymbolResolver() *Service {
	if s.pool != nil {
		s.symbolResolver = NewSymbolResolver(s.pool.Pool)
	}
	return s
}

// WithAccountConnector sets the account connection manager for persistent MT links.
func (s *Service) WithAccountConnector(ac AccountConnector) *Service {
	s.acctConn = ac
	return s
}

// WithSyncDonePublisher sets the function to publish sync-done SSE events.
func (s *Service) WithSyncDonePublisher(fn func(accountID string)) *Service {
	s.publishSyncDoneFn = fn
	return s
}

// WithSyncWorker sets the order sync worker for full/incremental sync.
func (s *Service) WithSyncWorker(sw OrderSyncer) *Service {
	s.syncWorker = sw
	return s
}

// WithHistoryRepo sets the local order-history repository.
func (s *Service) WithHistoryRepo(r HistoryOrderRepo) *Service {
	s.historyRepo = r
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
		return ErrSessionExpired
	}
	return s.pool.SetTenant(ctx, tenantID)
}
