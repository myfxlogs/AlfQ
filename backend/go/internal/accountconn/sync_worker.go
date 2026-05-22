// Package accountconn — order sync worker for historical order local persistence.
package accountconn

import (
	"context"
	"fmt"
	"time"

	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/alfq/backend/go/internal/mthub"
	"github.com/alfq/backend/go/internal/oms/repo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	orderSyncFullTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "order_sync_full_total",
		Help: "Total number of full order syncs completed.",
	})
	orderSyncIncrTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "order_sync_incr_total",
		Help: "Total number of incremental order syncs completed.",
	})
	orderSyncDeltaCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "order_sync_delta_count",
		Help: "Number of orders changed in the most recent sync.",
	})
	// mt5ProfitMismatchTotal counts events where the MT5 OnOrderProfit
	// stream-derived floating profit (Equity-Balance) differs from the
	// canonical AccountSummary.Profit by > $1. A non-zero value indicates
	// either Credit on the account or a broker-side mismatch between the
	// stream and terminal ACCOUNT_PROFIT — diagnostic only.
	mt5ProfitMismatchTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mt5_profit_mismatch_total",
		Help: "Count of MT5 stream-derived profit vs AccountSummary.Profit mismatches (>$1).",
	})
	// orderSyncLagSeconds = promauto.NewGauge(prometheus.GaugeOpts{
	// 	Name: "order_sync_lag_seconds",
	// 	Help: "Seconds between OnOrderUpdate event timestamp and DB write completion.",
	// })
)

// SyncWorker orchestrates full and incremental order history sync.
type SyncWorker struct {
	pool        *pg.Pool
	repo        *repo.HistoryOrderRepo
	log         *zap.Logger
	manager     *Manager
	mthubClient *mthub.Client
}

// NewSyncWorker creates a sync worker backed by the given pool + repo.
func NewSyncWorker(pool *pg.Pool, r *repo.HistoryOrderRepo, log *zap.Logger) *SyncWorker {
	return &SyncWorker{pool: pool, repo: r, log: log}
}

// SetManager binds the sync worker to the account connection manager
// so it can reuse live sessions and the mthub client.
func (w *SyncWorker) SetManager(m *Manager) {
	w.manager = m
	w.mthubClient = m.mthubClient
}

// SyncState holds per-account sync bookkeeping.
type SyncState struct {
	AccountID      string
	LastFullSyncAt *time.Time
	LastIncrSyncAt *time.Time
	SyncStatus     string
	LastError      string
	TotalSynced    int
}

// FullSync performs a full historical order sync for the given account.
// It fetches orders in monthly windows to avoid huge single payloads.
func (w *SyncWorker) FullSync(ctx context.Context, accountID string) error {
	if err := w.setSyncStatus(ctx, accountID, "syncing", ""); err != nil {
		return err
	}
	start := time.Now()

	// Resolve account metadata + connection details
	var tenantID, platform, login, password, server string
	err := w.pool.QueryRow(ctx,
		`SELECT tenant_id, platform, login, password, server FROM accounts WHERE id = $1`, accountID,
	).Scan(&tenantID, &platform, &login, &password, &server)
	if err != nil {
		_ = w.setSyncStatus(ctx, accountID, "error", err.Error())
		return fmt.Errorf("lookup account: %w", err)
	}

	// Resolve mtapi gateway address: try live Manager first, fallback to defaults.
	mtapiAddr := w.resolveGatewayAddr(platform)
	if mtapiAddr == "" {
		_ = w.setSyncStatus(ctx, accountID, "error", "no gateway address for platform "+platform)
		return fmt.Errorf("sync worker: no gateway addr for %s", platform)
	}

	// Default window: 10 years ago → now.
	windowEnd := time.Now()
	windowStart := windowEnd.AddDate(-10, 0, 0)

	total := 0

	// Batch by month to avoid huge payloads
	for windowStart.Before(windowEnd) {
		chunkEnd := windowStart.AddDate(0, 1, 0)
		if chunkEnd.After(windowEnd) {
			chunkEnd = windowEnd
		}

		var orders []*HistoryOrderInfo
		if w.mthubClient != nil {
			mthubOrders, err := w.mthubClient.OrderHistory(ctx, accountID,
				windowStart.Format(time.RFC3339), chunkEnd.Format(time.RFC3339))
			if err == nil {
				for _, o := range mthubOrders {
					orders = append(orders, &HistoryOrderInfo{
						Ticket: o.Ticket, Symbol: o.Symbol, Type: o.Side, Lots: o.Lots,
						OpenPrice: o.OpenPrice, ClosePrice: o.ClosePrice,
						Profit: o.Profit, Swap: o.Swap, Commission: o.Commission,
						OpenTime: o.OpenTime, CloseTime: o.CloseTime,
					})
				}
			} else {
				w.log.Warn("full sync chunk via mthub failed",
					zap.String("account_id", accountID),
					zap.Error(err),
				)
				windowStart = chunkEnd
				continue
			}
		} else {
			w.log.Warn("full sync: mthub client not available, skipping chunk",
				zap.String("account_id", accountID),
				zap.Time("from", windowStart),
				zap.Time("to", chunkEnd),
			)
			windowStart = chunkEnd
			continue
		}
		if len(orders) > 0 {
			repoOrders := make([]*repo.HistoryOrder, 0, len(orders))
			for _, o := range orders {
				repoOrders = append(repoOrders, repo.ToHistoryOrder(tenantID, accountID, &repo.HistoryOrderInput{
					Ticket: o.Ticket, Symbol: o.Symbol, Type: o.Type, Lots: o.Lots,
					OpenPrice: o.OpenPrice, ClosePrice: o.ClosePrice,
					Profit: o.Profit, Swap: o.Swap, Commission: o.Commission,
					OpenTime: o.OpenTime, CloseTime: o.CloseTime,
				}, "closed"))
			}
			if _, err := w.repo.BatchUpsert(ctx, tenantID, repoOrders); err != nil {
				w.log.Warn("full sync upsert failed", zap.Error(err))
			} else {
				total += len(orders)
			}
		}
		windowStart = chunkEnd
		// Throttle between chunks to avoid hammering the gateway
		select {
		case <-ctx.Done():
			_ = w.setSyncStatus(ctx, accountID, "error", ctx.Err().Error())
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}

	orderSyncFullTotal.Inc()
	orderSyncDeltaCount.Set(float64(total))
	w.log.Info("full sync completed",
		zap.String("account_id", accountID),
		zap.Int("total", total),
		zap.Duration("elapsed", time.Since(start)),
	)

	if err := w.updateSyncState(ctx, accountID, total); err != nil {
		w.log.Warn("failed to update sync state", zap.Error(err))
	}
	return nil
}

// resolveGatewayAddr returns the mtapi gateway address for the given platform.
func (w *SyncWorker) resolveGatewayAddr(platform string) string {
	switch platform {
	case "mt5":
		return "mt5grpc3.mtapi.io:443"
	case "mt4":
		return "mt4grpc3.mtapi.io:443"
	default:
		return ""
	}
}

// IncrSync performs an incremental sync for the given time window.
// Typically called after reconnection or OnOrderUpdate events.
func (w *SyncWorker) IncrSync(ctx context.Context, accountID string, from, to time.Time) ([]*repo.HistoryOrder, error) {
	if w.manager == nil {
		return nil, fmt.Errorf("sync worker: manager not bound")
	}
	var tenantID string
	_ = w.pool.QueryRow(ctx,
		`SELECT tenant_id FROM accounts WHERE id = $1`, accountID,
	).Scan(&tenantID)
	if tenantID == "" {
		return nil, fmt.Errorf("account not found: %s", accountID)
	}

	var changed []*repo.HistoryOrder
	if w.mthubClient == nil {
		return nil, fmt.Errorf("incr sync: no mthub client")
	}
	mthubOrders, err := w.mthubClient.OrderHistory(ctx, accountID,
		from.Format(time.RFC3339), to.Format(time.RFC3339))
	if err != nil {
		_ = w.setSyncStatus(ctx, accountID, "error", err.Error())
		return nil, err
	}
	if len(mthubOrders) > 0 {
		repoOrders := make([]*repo.HistoryOrder, 0, len(mthubOrders))
		for _, o := range mthubOrders {
			repoOrders = append(repoOrders, repo.ToHistoryOrder(tenantID, accountID, &repo.HistoryOrderInput{
				Ticket: o.Ticket, Symbol: o.Symbol, Type: o.Side, Lots: o.Lots,
				OpenPrice: o.OpenPrice, ClosePrice: o.ClosePrice,
				Profit: o.Profit, Swap: o.Swap, Commission: o.Commission,
				OpenTime: o.OpenTime, CloseTime: o.CloseTime,
			}, "closed"))
		}
		changed, err = w.repo.BatchUpsert(ctx, tenantID, repoOrders)
		if err != nil {
			_ = w.setSyncStatus(ctx, accountID, "error", err.Error())
			return nil, err
		}
	}
	if err != nil {
		_ = w.setSyncStatus(ctx, accountID, "error", err.Error())
		return nil, err
	}

	orderSyncIncrTotal.Inc()
	_ = w.setIncrSyncState(ctx, accountID)
	return changed, nil
}

// RecentSync pulls the last 5 minutes and upserts. Used by OnOrderUpdate handler.
func (w *SyncWorker) RecentSync(ctx context.Context, accountID string) ([]*repo.HistoryOrder, error) {
	now := time.Now()
	return w.IncrSync(ctx, accountID, now.Add(-5*time.Minute), now)
}

// GetSyncState reads the current sync state for an account.
func (w *SyncWorker) GetSyncState(ctx context.Context, accountID string) (*SyncState, error) {
	var s SyncState
	var lastFull, lastIncr *time.Time
	err := w.pool.QueryRow(ctx, `
		SELECT account_id, last_full_sync_at, last_incr_sync_at, sync_status, last_error, total_synced
		FROM account_sync_state WHERE account_id = $1
	`, accountID).Scan(&s.AccountID, &lastFull, &lastIncr, &s.SyncStatus, &s.LastError, &s.TotalSynced)
	if err != nil {
		return nil, err
	}
	s.LastFullSyncAt = lastFull
	s.LastIncrSyncAt = lastIncr
	return &s, nil
}

// internal helpers

func (w *SyncWorker) setSyncStatus(ctx context.Context, accountID, status, lastErr string) error {
	_, err := w.pool.Exec(ctx, `
		INSERT INTO account_sync_state (account_id, sync_status, last_error, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (account_id) DO UPDATE
		SET sync_status = EXCLUDED.sync_status,
		    last_error  = EXCLUDED.last_error,
		    updated_at  = now()
	`, accountID, status, lastErr)
	return err
}

func (w *SyncWorker) updateSyncState(ctx context.Context, accountID string, total int) error {
	_, err := w.pool.Exec(ctx, `
		INSERT INTO account_sync_state (account_id, last_full_sync_at, sync_status, last_error, total_synced, updated_at)
		VALUES ($1, now(), 'idle', '', $2, now())
		ON CONFLICT (account_id) DO UPDATE
		SET last_full_sync_at = now(),
		    sync_status       = 'idle',
		    last_error        = '',
		    total_synced      = $2,
		    updated_at        = now()
	`, accountID, total)
	return err
}

func (w *SyncWorker) setIncrSyncState(ctx context.Context, accountID string) error {
	_, err := w.pool.Exec(ctx, `
		INSERT INTO account_sync_state (account_id, last_incr_sync_at, sync_status, last_error, updated_at)
		VALUES ($1, now(), 'idle', '', now())
		ON CONFLICT (account_id) DO UPDATE
		SET last_incr_sync_at = now(),
		    sync_status       = 'idle',
		    last_error        = '',
		    updated_at        = now()
	`, accountID)
	return err
}
