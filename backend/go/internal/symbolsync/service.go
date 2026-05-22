// Package symbolsync — service entry point.
package symbolsync

import (
	"context"
	"fmt"
	"strings"

	"github.com/alfq/backend/go/internal/mthub"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Service orchestrates symbol sync for a broker account.
type Service struct {
	repo *Repo
	log  *zap.Logger
}

// NewService creates a symbol sync service.
func NewService(pool *pgxpool.Pool, log *zap.Logger) *Service {
	return &Service{repo: NewRepo(pool), log: log}
}

// SyncParams holds the data needed to sync symbols for an account.
type SyncParams struct {
	BrokerID  string // broker UUID from accounts.broker_id
	Platform  string // "MT4" or "MT5"
	SessionID string // gRPC session token
	Conn      *grpc.ClientConn
}

// Sync pulls symbol metadata and upserts to broker_symbols.
// Dispatch per platform: MT4 or MT5 path.
func (s *Service) Sync(ctx context.Context, params SyncParams) error {
	platform := strings.ToUpper(params.Platform)
	s.log.Info("symbol sync started",
		zap.String("broker_id", params.BrokerID),
		zap.String("platform", platform),
	)

	var symbols []BrokerSymbol
	var err error

	switch platform {
	case "MT5":
		symbols, err = FetchMT5Symbols(ctx, params.Conn, params.SessionID, params.BrokerID, s.log)
	case "MT4":
		symbols, err = FetchMT4Symbols(ctx, params.Conn, params.SessionID, params.BrokerID, s.log)
	default:
		return fmt.Errorf("symbolsync: unsupported platform %q", platform)
	}

	if err != nil {
		return fmt.Errorf("symbolsync: fetch: %w", err)
	}

	if err := s.repo.UpsertSymbols(ctx, symbols); err != nil {
		return fmt.Errorf("symbolsync: upsert: %w", err)
	}

	s.log.Info("symbol sync complete",
		zap.Int("total", len(symbols)),
		zap.String("broker_id", params.BrokerID),
	)
	return nil
}

// SyncViaMthub syncs symbols through the MT Session Hub.
// Falls back to the caller providing a direct conn if mthub isn't available.
func (s *Service) SyncViaMthub(ctx context.Context, client *mthub.Client, brokerID, platform, accountID string) error {
	s.log.Info("symbol sync via mthub started",
		zap.String("broker_id", brokerID),
		zap.String("account_id", accountID),
		zap.String("platform", platform),
	)
	// mthub.SymbolParamsMany is stubbed; delegate to direct Sync when a conn is available.
	// Once MH-4 wiring completes, this will use the mthub RPC instead.
	return fmt.Errorf("symbolsync: SyncViaMthub requires direct conn; use Sync() with the gRPC connection instead")
}
