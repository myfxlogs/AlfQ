// Package oms/reconciler — broker reconciliation: compares local PG orders
// with broker-side ticket state, transitions stale orders, and records diffs.
package oms

import (
	"context"
	"fmt"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/alfq/backend/go/internal/mthub"
	"github.com/alfq/backend/go/internal/oms/repo"
	"go.uber.org/zap"
)

// Reconciler compares local orders with broker state every 30s.
type Reconciler struct {
	pool        *pg.Pool
	orders      *repo.OrderRepo
	mthubClient *mthub.Client
	log         *zap.Logger
}

// NewReconciler creates a reconciler.
func NewReconciler(pool *pg.Pool, orders *repo.OrderRepo, mthubClient *mthub.Client, log *zap.Logger) *Reconciler {
	return &Reconciler{pool: pool, orders: orders, mthubClient: mthubClient, log: log}
}

// Run starts the reconciliation loop.
func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

// accountOrders holds PG state for one account.
type accountOrders struct {
	tenantID  string
	accountID string
	orders    []*pgOrder
}

type pgOrder struct {
	orderID      string
	tenantID     string
	accountID    string
	brokerTicket string
	state        int32
	symbol       string
}

func (r *Reconciler) reconcile(ctx context.Context) {
	// 1. Pull all non-terminal orders from PG.
	accMap, err := r.loadActiveOrders(ctx)
	if err != nil {
		r.log.Warn("reconciler: load active orders", zap.Error(err))
		return
	}
	if len(accMap) == 0 {
		return
	}

	// 2. For each account, pull broker-side open positions via mthub.
	for accountID, ao := range accMap {
		r.reconcileAccount(ctx, accountID, ao)
	}
}

func (r *Reconciler) loadActiveOrders(ctx context.Context) (map[string]*accountOrders, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT o.order_id, o.tenant_id, o.account_id, o.broker_ticket, o.state, o.symbol
		 FROM orders o
		 WHERE o.state NOT IN ($1,$2,$3,$4,$5)
		   AND o.broker_ticket IS NOT NULL AND o.broker_ticket != ''`,
		int32(pb.OrderState_ORDER_STATE_FILLED),
		int32(pb.OrderState_ORDER_STATE_CANCELLED),
		int32(pb.OrderState_ORDER_STATE_REJECTED),
		int32(pb.OrderState_ORDER_STATE_FAILED),
		int32(pb.OrderState_ORDER_STATE_EXPIRED),
	)
	if err != nil {
		return nil, fmt.Errorf("query active orders: %w", err)
	}
	defer rows.Close()

	accMap := make(map[string]*accountOrders)
	for rows.Next() {
		var o pgOrder
		if err := rows.Scan(&o.orderID, &o.tenantID, &o.accountID, &o.brokerTicket, &o.state, &o.symbol); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		ao, ok := accMap[o.accountID]
		if !ok {
			ao = &accountOrders{tenantID: o.tenantID, accountID: o.accountID}
			accMap[o.accountID] = ao
		}
		ao.orders = append(ao.orders, &o)
	}
	return accMap, rows.Err()
}

func (r *Reconciler) reconcileAccount(ctx context.Context, accountID string, ao *accountOrders) {
	if r.mthubClient == nil {
		return
	}

	// Pull broker-side open positions via mthub.
	brokerOrders, err := r.mthubClient.OpenedOrders(ctx, accountID)
	if err != nil {
		r.log.Debug("reconciler: mthub opened orders",
			zap.String("account_id", accountID),
			zap.Error(err),
		)
		return
	}

	// Build broker ticket set for quick lookup.
	brokerByTicket := make(map[string]*mthub.OrderRecord)
	for _, bo := range brokerOrders {
		ticket := fmt.Sprintf("%d", bo.Ticket)
		brokerByTicket[ticket] = bo
	}

	// 3. Compare PG orders with broker state.
	for _, po := range ao.orders {
		bo, inBroker := brokerByTicket[po.brokerTicket]

		if !inBroker {
			// Order is in PG but not in broker open positions.
			// It may have been filled or cancelled. Transition to FILLED as
			// a conservative default — the order was accepted by the broker
			// and is no longer open.
			r.transitionIfValid(ctx, ao.tenantID, po, pb.OrderState_ORDER_STATE_FILLED, 0,
				"reconciliation", "order not found in broker open positions; assumed filled")
			continue
		}

		// Map broker-side state to PG state.
		brokerState := mapBrokerState(bo)
		if brokerState == pb.OrderState_ORDER_STATE_UNSPECIFIED {
			continue
		}

		pgState := pb.OrderState(po.state)
		if brokerState != pgState {
			r.transitionIfValid(ctx, ao.tenantID, po, brokerState, float64(bo.Lots),
				"reconciliation", fmt.Sprintf("broker state %s differs from PG state %s", brokerState, pgState))
		}
	}
}

// mapBrokerState maps mthub.OrderRecord to a pb.OrderState.
// OpenedOrders returns open positions which are always in WORKING or similar state.
func mapBrokerState(o *mthub.OrderRecord) pb.OrderState {
	// OpenedOrders returns currently open positions.
	// State from mthub may be empty string; default to WORKING.
	switch o.State {
	case "WORKING", "":
		return pb.OrderState_ORDER_STATE_WORKING
	default:
		return pb.OrderState_ORDER_STATE_UNSPECIFIED
	}
}

// transitionIfValid validates and executes a state transition, writing diffs to risk_events.
// All state changes MUST go through oms.Transition() per RC10 requirements.
func (r *Reconciler) transitionIfValid(ctx context.Context, tenantID string, po *pgOrder, next pb.OrderState, filledQty float64, ruleID, reason string) {
	current := pb.OrderState(po.state)
	if err := Transition(current, next); err != nil {
		// Invalid transition — log and skip.
		r.log.Debug("reconciler: skip invalid transition",
			zap.String("order_id", po.orderID),
			zap.String("from", current.String()),
			zap.String("to", next.String()),
			zap.Error(err),
		)
		return
	}

	// Update PG order state via oms.Transition already validated above.
	if err := r.orders.UpdateState(ctx, po.orderID, next, filledQty); err != nil {
		r.log.Warn("reconciler: update state failed",
			zap.String("order_id", po.orderID),
			zap.Error(err),
		)
		return
	}

	// Write reconciliation event to risk_events.
	r.writeReconciliationEvent(ctx, tenantID, po, next, ruleID, reason)

	r.log.Debug("reconciler: state transitioned",
		zap.String("order_id", po.orderID),
		zap.String("account_id", po.accountID),
		zap.String("ticket", po.brokerTicket),
		zap.String("from", current.String()),
		zap.String("to", next.String()),
	)
}

// writeReconciliationEvent inserts a diff record into risk_events for audit trail.
func (r *Reconciler) writeReconciliationEvent(ctx context.Context, tenantID string, po *pgOrder, next pb.OrderState, ruleID, reason string) {
	nowMs := time.Now().UnixMilli()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO risk_events (tenant_id, event_type, account_id, symbol, rule_id, reason, severity, ts_unix_ms)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		tenantID, "reconciliation", po.accountID, po.symbol, ruleID, reason, "P2", nowMs,
	)
	if err != nil {
		r.log.Warn("reconciler: write risk_event failed",
			zap.String("order_id", po.orderID),
			zap.Error(err),
		)
	}
}
