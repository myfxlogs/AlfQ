// Package oms — OrderExecutor: risk-validated order submission with SSE broadcast.
package oms

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/risksvc"
	"github.com/alfq/backend/go/internal/ssehub"
)

// OrderExecutor submits orders through risk checks and broker adapter.
type OrderExecutor struct {
	adapter BrokerAdapter
	risk    *risksvc.Engine
	sse     *ssehub.Hub
}

// NewOrderExecutor creates an order executor.
func NewOrderExecutor(adapter BrokerAdapter, risk *risksvc.Engine, sse *ssehub.Hub) *OrderExecutor {
	return &OrderExecutor{adapter: adapter, risk: risk, sse: sse}
}

// Submit runs risk check → submit → SSE broadcast.
func (e *OrderExecutor) Submit(ctx context.Context, req *pb.OrderRequest) (*BrokerResp, error) {
	// 1. Risk check
	result := e.risk.Check(ctx, req)
	if !result.Approved {
		e.broadcast("order_rejected", map[string]any{
			"symbol":   req.Symbol,
			"side":     req.Side.String(),
			"qty":      req.Qty,
			"reason":   result.Reason,
			"rule_id":  result.RuleId,
			"strategy": req.StrategyId,
		})
		return nil, fmt.Errorf("risk rejected: %s (rule: %s)", result.Reason, result.RuleId)
	}

	// 2. Submit to broker
	resp, err := e.adapter.Submit(ctx, req)
	if err != nil {
		e.broadcast("order_error", map[string]any{
			"symbol": req.Symbol,
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("oms submit: %w", err)
	}

	// 3. Broadcast SSE event
	e.broadcast("order_submitted", map[string]any{
		"ticket":   resp.Ticket,
		"symbol":   req.Symbol,
		"side":     req.Side.String(),
		"qty":      req.Qty,
		"state":    resp.State.String(),
		"strategy": req.StrategyId,
	})

	return resp, nil
}

func (e *OrderExecutor) broadcast(eventType string, data map[string]any) {
	if e.sse == nil {
		return
	}
	payload := map[string]any{
		"type":      eventType,
		"timestamp": time.Now().UnixMilli(),
		"data":      data,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return
	}
	e.sse.Broadcast(bytes)
}
