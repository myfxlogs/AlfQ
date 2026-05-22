// Package adminapi — Symbol Resolver (RS06).
// Validates that strategy symbols are tradable on the target broker before accepting specs.
package adminapi

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SymbolInfo holds the resolved broker symbol for a canonical name.
type SymbolInfo struct {
	SymbolRaw string
	Canonical string
	TradeMode int32 // 0=disabled, 1=long_only, 2=short_only, 3=full
}

// SymbolResolver resolves canonical symbols to broker-specific symbols.
type SymbolResolver struct {
	pool *pgxpool.Pool
}

// NewSymbolResolver creates a symbol resolver backed by PG.
func NewSymbolResolver(pool *pgxpool.Pool) *SymbolResolver {
	return &SymbolResolver{pool: pool}
}

// ResolveCanonical looks up a canonical symbol for a given account's broker.
func (r *SymbolResolver) ResolveCanonical(ctx context.Context, accountID, canonical string) (*SymbolInfo, bool, error) {
	if r.pool == nil {
		return nil, false, fmt.Errorf("pg not available")
	}
	var symbolRaw, canon string
	var tradeMode int32
	err := r.pool.QueryRow(ctx, `
		SELECT bs.symbol_raw, bs.canonical, bs.trade_mode
		FROM broker_symbols bs
		JOIN accounts a ON a.broker_id = bs.broker_id
		WHERE a.id = $1 AND bs.canonical = $2
		LIMIT 1
	`, accountID, canonical).Scan(&symbolRaw, &canon, &tradeMode)
	if err != nil {
		return nil, false, fmt.Errorf("symbol %s not found for account %s", canonical, accountID)
	}

	info := &SymbolInfo{SymbolRaw: symbolRaw, Canonical: canon, TradeMode: tradeMode}
	if tradeMode == 0 {
		return info, false, fmt.Errorf("symbol %s is disabled on broker (%s)", canonical, symbolRaw)
	}
	return info, true, nil
}

// ListSupportedCanonicals returns all tradeable canonical symbols for an account.
func (r *SymbolResolver) ListSupportedCanonicals(ctx context.Context, accountID string) ([]string, error) {
	if r.pool == nil {
		return nil, fmt.Errorf("pg not available")
	}
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT bs.canonical
		FROM broker_symbols bs
		JOIN accounts a ON a.broker_id = bs.broker_id
		WHERE a.id = $1 AND bs.trade_mode > 0
		ORDER BY bs.canonical
	`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list symbols: %w", err)
	}
	defer rows.Close()

	var canonicals []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			continue
		}
		canonicals = append(canonicals, c)
	}
	return canonicals, nil
}
