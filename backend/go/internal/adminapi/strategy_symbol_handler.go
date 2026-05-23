// Package adminapi — StrategySymbolService handler (M5).
package adminapi

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StrategySymbolHandler implements the StrategySymbolService ConnectRPC handler.
type StrategySymbolHandler struct {
	pool *pgxpool.Pool
}

// NewStrategySymbolHandler creates a new handler.
func NewStrategySymbolHandler(pool *pgxpool.Pool) *StrategySymbolHandler {
	return &StrategySymbolHandler{pool: pool}
}

// ListAvailableCanonicals returns canonical symbols available for a tenant.
func (h *StrategySymbolHandler) ListAvailableCanonicals(ctx context.Context, req *connect.Request[pb.ListAvailableCanonicalsRequest]) (*connect.Response[pb.ListAvailableCanonicalsResponse], error) {
	if h.pool == nil {
		return nil, fmt.Errorf("pg not available")
	}
	rows, err := h.pool.Query(ctx,
		`SELECT cs.canonical, cs.asset_class, cs.description, tcw.enabled
		 FROM tenant_canonical_whitelist tcw
		 JOIN canonical_symbols cs ON cs.canonical = tcw.canonical
		 WHERE tcw.tenant_id = $1 AND tcw.enabled = true AND cs.enabled = true
		 ORDER BY cs.canonical`,
		req.Msg.TenantId,
	)
	if err != nil {
		return nil, fmt.Errorf("list available canonicals: %w", err)
	}
	defer rows.Close()

	var canonicals []*pb.CanonicalInfo
	for rows.Next() {
		var c pb.CanonicalInfo
		if err := rows.Scan(&c.Canonical, &c.AssetClass, &c.Description, &c.Enabled); err != nil {
			continue
		}
		canonicals = append(canonicals, &c)
	}
	return connect.NewResponse(&pb.ListAvailableCanonicalsResponse{Canonicals: canonicals}), nil
}

// ResolveCanonicalsForAccount resolves canonical symbols to broker-specific names.
func (h *StrategySymbolHandler) ResolveCanonicalsForAccount(ctx context.Context, req *connect.Request[pb.ResolveCanonicalsRequest]) (*connect.Response[pb.ResolveCanonicalsResponse], error) {
	if h.pool == nil {
		return nil, fmt.Errorf("pg not available")
	}
	var resolutions []*pb.CanonicalResolution
	for _, canonical := range req.Msg.Canonicals {
		var symbolRaw string
		var tradeMode int32
		err := h.pool.QueryRow(ctx,
			`SELECT bs.symbol_raw, bs.trade_mode
			 FROM broker_symbols bs
			 JOIN accounts a ON a.broker_id = bs.broker_id
			 WHERE a.id = $1 AND bs.canonical = $2
			 LIMIT 1`,
			req.Msg.AccountId, canonical,
		).Scan(&symbolRaw, &tradeMode)
		if err != nil {
			resolutions = append(resolutions, &pb.CanonicalResolution{
				Canonical: canonical,
				Tradable:  false,
			})
			continue
		}
		resolutions = append(resolutions, &pb.CanonicalResolution{
			Canonical: canonical,
			SymbolRaw: symbolRaw,
			Tradable:  tradeMode > 0,
			TradeMode: tradeMode,
		})
	}
	return connect.NewResponse(&pb.ResolveCanonicalsResponse{Resolutions: resolutions}), nil
}

// UpdateStrategySymbols replaces a strategy's canonical symbol whitelist.
func (h *StrategySymbolHandler) UpdateStrategySymbols(ctx context.Context, req *connect.Request[pb.UpdateStrategySymbolsRequest]) (*connect.Response[pb.UpdateStrategySymbolsResponse], error) {
	if h.pool == nil {
		return nil, fmt.Errorf("pg not available")
	}
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get current set
	currentRows, err := tx.Query(ctx,
		`SELECT canonical FROM strategy_symbols WHERE strategy_id = $1`,
		req.Msg.StrategyId,
	)
	if err != nil {
		return nil, fmt.Errorf("query current: %w", err)
	}
	current := make(map[string]bool)
	for currentRows.Next() {
		var c string
		if err := currentRows.Scan(&c); err == nil {
			current[c] = true
		}
	}
	currentRows.Close()

	// Build new set
	desired := make(map[string]bool)
	for _, c := range req.Msg.Canonicals {
		desired[c] = true
	}

	var added, removed int32

	// Remove entries not in desired
	for c := range current {
		if !desired[c] {
			_, err := tx.Exec(ctx,
				`DELETE FROM strategy_symbols WHERE strategy_id = $1 AND canonical = $2`,
				req.Msg.StrategyId, c,
			)
			if err == nil {
				removed++
			}
		}
	}

	// Add entries not in current
	for c := range desired {
		if !current[c] {
			_, err := tx.Exec(ctx,
				`INSERT INTO strategy_symbols (strategy_id, canonical)
				 VALUES ($1, $2)
				 ON CONFLICT (strategy_id, canonical) DO UPDATE SET enabled = true`,
				req.Msg.StrategyId, c,
			)
			if err == nil {
				added++
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return connect.NewResponse(&pb.UpdateStrategySymbolsResponse{Added: added, Removed: removed}), nil
}
