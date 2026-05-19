// Package oms/reconciler — broker reconciliation.
package oms

import (
	"context"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/oms/repo"
)

// Reconciler compares local orders with broker state every 60s.
type Reconciler struct {
	orders  *repo.OrderRepo
	adapter BrokerAdapter
}

// NewReconciler creates a reconciler.
func NewReconciler(orders *repo.OrderRepo, adapter BrokerAdapter) *Reconciler {
	return &Reconciler{orders: orders, adapter: adapter}
}

// Run starts the reconciliation loop.
func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
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

func (r *Reconciler) reconcile(ctx context.Context) {
	// TODO: pull orders from broker via adapter.Query,
	// compare with PG, write diffs to risk_events
	_ = pb.OrderState_ORDER_STATE_FILLED
}
