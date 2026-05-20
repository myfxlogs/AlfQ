// Package accountconn — order sync worker for historical order local persistence.
package accountconn

import (
	"context"
	"fmt"
	"time"

	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/alfq/backend/go/internal/mdgateway/adapter/mtapi"
	"github.com/alfq/backend/go/internal/oms/repo"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// SyncWorker orchestrates full and incremental order history sync.
type SyncWorker struct {
	pool    *pg.Pool
	repo    *repo.HistoryOrderRepo
	log     *zap.Logger
	manager *Manager
}

// NewSyncWorker creates a sync worker backed by the given pool + repo.
func NewSyncWorker(pool *pg.Pool, r *repo.HistoryOrderRepo, log *zap.Logger) *SyncWorker {
	return &SyncWorker{pool: pool, repo: r, log: log}
}

// SetManager binds the sync worker to the account connection manager
// so it can reuse live sessions.
func (w *SyncWorker) SetManager(m *Manager) { w.manager = m }

// SyncState holds per-account sync bookkeeping.
type SyncState struct {
	AccountID       string
	LastFullSyncAt  *time.Time
	LastIncrSyncAt  *time.Time
	SyncStatus      string
	LastError       string
	TotalSynced     int
}

// FullSync performs a full historical order sync for the given account.
// It fetches orders in monthly windows to avoid huge single payloads.
func (w *SyncWorker) FullSync(ctx context.Context, accountID string) error {
	if err := w.setSyncStatus(ctx, accountID, "syncing", ""); err != nil {
		return err
	}
	start := time.Now()

	// Resolve account metadata
	var tenantID, platform string
	err := w.pool.QueryRow(ctx,
		`SELECT tenant_id, platform FROM accounts WHERE id = $1`, accountID,
	).Scan(&tenantID, &platform)
	if err != nil {
		w.setSyncStatus(ctx, accountID, "error", err.Error())
		return fmt.Errorf("lookup account: %w", err)
	}

	// Default window: account creation / 1 year ago → now
	var accountCreated time.Time
	_ = w.pool.QueryRow(ctx,
		`SELECT COALESCE(created_at, now() - interval '1 year') FROM accounts WHERE id = $1`, accountID,
	).Scan(&accountCreated)
	if accountCreated.IsZero() {
		accountCreated = time.Now().AddDate(-1, 0, 0)
	}

	// Use live session via Manager if available
	if w.manager == nil {
		w.setSyncStatus(ctx, accountID, "error", "manager not bound")
		return fmt.Errorf("sync worker: manager not bound")
	}

	total := 0
	windowEnd := time.Now()
	windowStart := accountCreated

	// Batch by month to avoid huge payloads
	for windowStart.Before(windowEnd) {
		chunkEnd := windowStart.AddDate(0, 1, 0)
		if chunkEnd.After(windowEnd) {
			chunkEnd = windowEnd
		}

		err := w.manager.WithLiveSession(accountID, func(conn *grpc.ClientConn, sessionID, plat string) error {
			orders, err := mtapi.FetchOrderHistory(ctx, conn, plat, sessionID,
				windowStart.Format(time.RFC3339), chunkEnd.Format(time.RFC3339))
			if err != nil {
				return err
			}
			if len(orders) == 0 {
				return nil
			}
			repoOrders := make([]*repo.HistoryOrder, 0, len(orders))
			for _, o := range orders {
				repoOrders = append(repoOrders, repo.ToHistoryOrder(tenantID, accountID, o, "closed"))
			}
			_, err = w.repo.BatchUpsert(ctx, tenantID, repoOrders)
			if err != nil {
				return err
			}
			total += len(orders)
			return nil
		})
		if err != nil {
			w.log.Warn("full sync chunk failed",
				zap.String("account_id", accountID),
				zap.Time("from", windowStart),
				zap.Time("to", chunkEnd),
				zap.Error(err),
			)
			// Continue with next chunk; do not abort entire sync.
		}

		windowStart = chunkEnd
		// Throttle between chunks to avoid hammering the gateway
		select {
		case <-ctx.Done():
			w.setSyncStatus(ctx, accountID, "error", ctx.Err().Error())
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}

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

// IncrSync performs an incremental sync for the given time window.
// Typically called after reconnection or OnOrderUpdate events.
func (w *SyncWorker) IncrSync(ctx context.Context, accountID string, from, to time.Time) error {
	if w.manager == nil {
		return fmt.Errorf("sync worker: manager not bound")
	}
	var tenantID string
	_ = w.pool.QueryRow(ctx,
		`SELECT tenant_id FROM accounts WHERE id = $1`, accountID,
	).Scan(&tenantID)
	if tenantID == "" {
		return fmt.Errorf("account not found: %s", accountID)
	}

	err := w.manager.WithLiveSession(accountID, func(conn *grpc.ClientConn, sessionID, plat string) error {
		orders, err := mtapi.FetchOrderHistory(ctx, conn, plat, sessionID,
			from.Format(time.RFC3339), to.Format(time.RFC3339))
		if err != nil {
			return err
		}
		if len(orders) == 0 {
			return nil
		}
		repoOrders := make([]*repo.HistoryOrder, 0, len(orders))
		for _, o := range orders {
			repoOrders = append(repoOrders, repo.ToHistoryOrder(tenantID, accountID, o, "closed"))
		}
		_, err = w.repo.BatchUpsert(ctx, tenantID, repoOrders)
		return err
	})
	if err != nil {
		w.setSyncStatus(ctx, accountID, "error", err.Error())
		return err
	}

	_ = w.setIncrSyncState(ctx, accountID)
	return nil
}

// RecentSync pulls the last 5 minutes and upserts. Used by OnOrderUpdate handler.
func (w *SyncWorker) RecentSync(ctx context.Context, accountID string) error {
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
