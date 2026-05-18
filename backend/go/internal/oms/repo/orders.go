// Package oms/repo — PG order repository.
package repo

import (
	"context"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/db/pg"
)

// OrderRepo provides PostgreSQL access for orders.
type OrderRepo struct {
	pool *pg.Pool
}

// NewOrderRepo creates an order repository.
func NewOrderRepo(pool *pg.Pool) *OrderRepo { return &OrderRepo{pool: pool} }

// Insert creates a new order row.
func (r *OrderRepo) Insert(ctx context.Context, o *pb.Order) error {
	if err := r.pool.SetTenant(ctx, o.TenantId); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO orders (order_id, tenant_id, account_id, strategy_id, client_order_id,
		 broker_ticket, symbol, side, type, state, price, stop_price, qty, filled_qty)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		o.OrderId, o.TenantId, o.AccountId, o.StrategyId, o.ClientOrderId,
		o.BrokerTicket, o.Symbol, int32(o.Side), int32(o.Type), int32(o.State),
		o.GetPrice().GetValue(), o.GetStopPrice().GetValue(), o.Qty, o.FilledQty,
	)
	return err
}

// UpdateState transitions an order state and updates filled quantity.
func (r *OrderRepo) UpdateState(ctx context.Context, orderID string, state pb.OrderState, filledQty float64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE orders SET state = $1, filled_qty = $2, updated_ts_ms = extract(epoch from now())*1000 WHERE order_id = $3`,
		int32(state), filledQty, orderID,
	)
	return err
}

// FindByClientOrderID looks up an order by idempotency key.
func (r *OrderRepo) FindByClientOrderID(ctx context.Context, accountID, clientOrderID string) (*pb.Order, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT order_id, tenant_id, account_id, strategy_id, client_order_id, broker_ticket,
		 symbol, side, type, state, price, stop_price, qty, filled_qty
		 FROM orders WHERE account_id = $1 AND client_order_id = $2`,
		accountID, clientOrderID,
	)
	var o pb.Order
	err := row.Scan(&o.OrderId, &o.TenantId, &o.AccountId, &o.StrategyId, &o.ClientOrderId,
		&o.BrokerTicket, &o.Symbol, &o.Side, &o.Type, &o.State,
		&o.Price, &o.StopPrice, &o.Qty, &o.FilledQty,
	)
	if err != nil {
		return nil, err
	}
	return &o, nil
}
