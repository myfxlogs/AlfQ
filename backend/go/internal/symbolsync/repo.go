// Package symbolsync — PostgreSQL upsert for broker_symbols.
package symbolsync

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo writes broker_symbols to PostgreSQL.
type Repo struct {
	pool *pgxpool.Pool
}

// NewRepo creates a Repo.
func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// UpsertSymbols bulk-upserts broker symbols.
// Uses ON CONFLICT (broker_id, symbol_raw) DO UPDATE.
func (r *Repo) UpsertSymbols(ctx context.Context, symbols []BrokerSymbol) error {
	for _, s := range symbols {
		rawJSON, _ := json.Marshal(s.RawPayload)
		sessQuote, _ := json.Marshal(s.SessionsQuote)
		sessTrade, _ := json.Marshal(s.SessionsTrade)
		_, err := r.pool.Exec(ctx, `
			INSERT INTO broker_symbols (
				broker_id, symbol_raw, canonical,
				digits, point, tick_size, tick_value, contract_size,
				min_lot, max_lot, lot_step,
				margin_initial, margin_currency, profit_currency,
				swap_long, swap_short, swap_mode, swap_rollover_day,
				trade_mode, description,
				sessions_quote, sessions_trade, server_timezone,
				raw_payload, partial, updated_at
			) VALUES (
				$1,$2,$3, $4,$5,$6,$7,$8,
				$9,$10,$11, $12,$13,$14,
				$15,$16,$17,$18,
				$19,$20,
				$21,$22,$23,
				$24,$25, now()
			) ON CONFLICT (broker_id, symbol_raw) DO UPDATE SET
				canonical=EXCLUDED.canonical,
				digits=EXCLUDED.digits,
				point=EXCLUDED.point,
				tick_size=EXCLUDED.tick_size,
				tick_value=EXCLUDED.tick_value,
				contract_size=EXCLUDED.contract_size,
				min_lot=EXCLUDED.min_lot,
				max_lot=EXCLUDED.max_lot,
				lot_step=EXCLUDED.lot_step,
				margin_initial=EXCLUDED.margin_initial,
				margin_currency=EXCLUDED.margin_currency,
				profit_currency=EXCLUDED.profit_currency,
				swap_long=EXCLUDED.swap_long,
				swap_short=EXCLUDED.swap_short,
				swap_mode=EXCLUDED.swap_mode,
				swap_rollover_day=EXCLUDED.swap_rollover_day,
				trade_mode=EXCLUDED.trade_mode,
				description=EXCLUDED.description,
				sessions_quote=EXCLUDED.sessions_quote,
				sessions_trade=EXCLUDED.sessions_trade,
				server_timezone=EXCLUDED.server_timezone,
				raw_payload=EXCLUDED.raw_payload,
				partial=EXCLUDED.partial,
				updated_at=now()
		`, s.BrokerID, s.SymbolRaw, s.Canonical,
			s.Digits, s.Point, s.TickSize, s.TickValue, s.ContractSize,
			s.MinLot, s.MaxLot, s.LotStep,
			s.MarginInitial, s.MarginCurrency, s.ProfitCurrency,
			s.SwapLong, s.SwapShort, s.SwapMode, s.SwapRolloverDay,
			s.TradeMode, s.Description,
			sessQuote, sessTrade, s.ServerTimezone,
			rawJSON, s.Partial,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
