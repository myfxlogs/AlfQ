// Package repo — PG order-history repository for local MT order sync.
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/alfq/backend/go/internal/mdgateway/adapter/mtapi"
)

// HistoryOrder represents a row in orders_history.
type HistoryOrder struct {
	ID         string
	TenantID   string
	AccountID  string
	Ticket     int64
	Symbol     string
	Side       string
	Lots       float64
	OpenPrice  float64
	ClosePrice float64
	Profit     float64
	Swap       float64
	Commission float64
	OpenTime   time.Time
	CloseTime  *time.Time // nil when not closed
	State      string
	RawPayload map[string]interface{}
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UpsertResult tells whether the row was inserted (true) or updated (false).
type UpsertResult struct {
	Inserted bool
	ID       string
}

// HistoryOrderRepo provides PostgreSQL access for orders_history.
type HistoryOrderRepo struct {
	pool *pg.Pool
}

// NewHistoryOrderRepo creates an order-history repository.
func NewHistoryOrderRepo(pool *pg.Pool) *HistoryOrderRepo {
	return &HistoryOrderRepo{pool: pool}
}

// Upsert inserts or updates an order row using (account_id, ticket) as conflict key.
// The close_time version guard prevents stale events from overwriting newer state.
func (r *HistoryOrderRepo) Upsert(ctx context.Context, tenantID string, o *HistoryOrder) (*UpsertResult, error) {
	if err := r.pool.SetTenant(ctx, tenantID); err != nil {
		return nil, err
	}

	var rawPayload any
	if o.RawPayload != nil {
		rawPayload = o.RawPayload
	}

	var result UpsertResult
	var ctTime any
	if o.CloseTime != nil {
		ctTime = *o.CloseTime
	} else {
		ctTime = nil
	}

	err := r.pool.QueryRow(ctx, `
		INSERT INTO orders_history
			(tenant_id, account_id, ticket, symbol, side, lots,
			 open_price, close_price, profit, swap, commission,
			 open_time, close_time, state, raw_payload, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, now())
		ON CONFLICT (account_id, ticket) DO UPDATE
		SET symbol      = EXCLUDED.symbol,
		    side        = EXCLUDED.side,
		    lots        = EXCLUDED.lots,
		    open_price  = EXCLUDED.open_price,
		    close_price = EXCLUDED.close_price,
		    profit      = EXCLUDED.profit,
		    swap        = EXCLUDED.swap,
		    commission  = EXCLUDED.commission,
		    open_time   = EXCLUDED.open_time,
		    close_time  = EXCLUDED.close_time,
		    state       = EXCLUDED.state,
		    raw_payload = EXCLUDED.raw_payload,
		    updated_at  = now()
		WHERE
		    orders_history.close_time IS NULL
		    OR (EXCLUDED.close_time IS NOT NULL AND EXCLUDED.close_time >= orders_history.close_time)
		RETURNING (xmax = 0) AS inserted, id
	`, o.TenantID, o.AccountID, o.Ticket, o.Symbol, o.Side, o.Lots,
		o.OpenPrice, o.ClosePrice, o.Profit, o.Swap, o.Commission,
		o.OpenTime, ctTime, o.State, rawPayload,
	).Scan(&result.Inserted, &result.ID)

	if err != nil {
		return nil, fmt.Errorf("history order upsert: %w", err)
	}
	return &result, nil
}

// List returns historical orders for an account, ordered by close_time DESC.
// If from or to are zero-value they are ignored.
func (r *HistoryOrderRepo) List(ctx context.Context, tenantID, accountID string, from, to time.Time) ([]*HistoryOrder, error) {
	if err := r.pool.SetTenant(ctx, tenantID); err != nil {
		return nil, err
	}

	var query string
	var args []any

	if from.IsZero() && to.IsZero() {
		query = `
			SELECT id, tenant_id, account_id, ticket, symbol, side, lots,
			       open_price, close_price, profit, swap, commission,
			       open_time, close_time, state, raw_payload, created_at, updated_at
			FROM orders_history
			WHERE account_id = $1
			ORDER BY close_time DESC NULLS LAST, ticket DESC
			LIMIT 1000
		`
		args = []any{accountID}
	} else if from.IsZero() {
		query = `
			SELECT id, tenant_id, account_id, ticket, symbol, side, lots,
			       open_price, close_price, profit, swap, commission,
			       open_time, close_time, state, raw_payload, created_at, updated_at
			FROM orders_history
			WHERE account_id = $1 AND close_time <= $2
			ORDER BY close_time DESC NULLS LAST, ticket DESC
			LIMIT 1000
		`
		args = []any{accountID, to}
	} else if to.IsZero() {
		query = `
			SELECT id, tenant_id, account_id, ticket, symbol, side, lots,
			       open_price, close_price, profit, swap, commission,
			       open_time, close_time, state, raw_payload, created_at, updated_at
			FROM orders_history
			WHERE account_id = $1 AND close_time >= $2
			ORDER BY close_time DESC NULLS LAST, ticket DESC
			LIMIT 1000
		`
		args = []any{accountID, from}
	} else {
		query = `
			SELECT id, tenant_id, account_id, ticket, symbol, side, lots,
			       open_price, close_price, profit, swap, commission,
			       open_time, close_time, state, raw_payload, created_at, updated_at
			FROM orders_history
			WHERE account_id = $1 AND close_time BETWEEN $2 AND $3
			ORDER BY close_time DESC NULLS LAST, ticket DESC
			LIMIT 1000
		`
		args = []any{accountID, from, to}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("history order list: %w", err)
	}
	defer rows.Close()

	var out []*HistoryOrder
	for rows.Next() {
		o := &HistoryOrder{}
		var closeTime *time.Time
		var rawPayload any
		err := rows.Scan(
			&o.ID, &o.TenantID, &o.AccountID, &o.Ticket, &o.Symbol, &o.Side, &o.Lots,
			&o.OpenPrice, &o.ClosePrice, &o.Profit, &o.Swap, &o.Commission,
			&o.OpenTime, &closeTime, &o.State, &rawPayload, &o.CreatedAt, &o.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("history order scan: %w", err)
		}
		o.CloseTime = closeTime
		if rawPayload != nil {
			// pgx returns JSONB as map[string]interface{} or []byte depending on driver
			// default is map[string]interface{}
			if m, ok := rawPayload.(map[string]interface{}); ok {
				o.RawPayload = m
			}
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("history order rows: %w", err)
	}
	return out, nil
}

// BatchUpsert performs UPSERT for multiple orders in a single transaction.
// Returns orders whose row actually changed (insert or effective update).
func (r *HistoryOrderRepo) BatchUpsert(ctx context.Context, tenantID string, orders []*HistoryOrder) ([]*HistoryOrder, error) {
	if err := r.pool.SetTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	changed := make([]*HistoryOrder, 0, len(orders))
	for _, o := range orders {
		res, err := r.Upsert(ctx, tenantID, o)
		if err != nil {
			return nil, err
		}
		_ = res // Inserted tracking retained for future use (xmax=0)
		changed = append(changed, o)
	}
	return changed, nil
}

// ToHistoryOrder converts an mtapi.HistoryOrderInfo into a repo HistoryOrder.
func ToHistoryOrder(tenantID, accountID string, info *mtapi.HistoryOrderInfo, state string) *HistoryOrder {
	if state == "" {
		state = "closed"
	}
	toTime := func(s string) time.Time {
		if s == "" {
			return time.Time{}
		}
		t, _ := time.Parse(time.RFC3339, s)
		return t
	}
	var ct *time.Time
	if info.CloseTime != "" {
		t := toTime(info.CloseTime)
		ct = &t
	}
	return &HistoryOrder{
		TenantID:   tenantID,
		AccountID:  accountID,
		Ticket:     info.Ticket,
		Symbol:     info.Symbol,
		Side:       info.Type,
		Lots:       info.Lots,
		OpenPrice:  info.OpenPrice,
		ClosePrice: info.ClosePrice,
		Profit:     info.Profit,
		Swap:       info.Swap,
		Commission: info.Commission,
		OpenTime:   toTime(info.OpenTime),
		CloseTime:  ct,
		State:      state,
		RawPayload: map[string]interface{}{
			"open_time_str":  info.OpenTime,
			"close_time_str": info.CloseTime,
		},
	}
}
