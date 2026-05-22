// Package oms — OrderExecutor: state-machine-driven order submission with PG persistence.
package oms

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/oms/repo"
	"github.com/alfq/backend/go/internal/risksvc"
	"github.com/alfq/backend/go/internal/ssehub"
)

// RiskEventWriter persists risk rejection events for audit and promotion gate.
type RiskEventWriter interface {
	Write(ctx context.Context, tenantID, accountID, strategyID, ruleID, reason, severity string, orderReq *pb.OrderRequest) error
}

// OrderExecutor submits orders through the full state machine:
// NEW → VALIDATED → RISK_APPROVED → SUBMITTED (or REJECTED/FAILED).
// Every state transition is persisted to the PG orders table.
type OrderExecutor struct {
	adapter       BrokerAdapter
	risk          *risksvc.Engine
	sse           *ssehub.Hub
	orderRepo     *repo.OrderRepo
	riskEventRepo RiskEventWriter
}

// NewOrderExecutor creates an order executor.
func NewOrderExecutor(adapter BrokerAdapter, risk *risksvc.Engine, sse *ssehub.Hub) *OrderExecutor {
	return &OrderExecutor{adapter: adapter, risk: risk, sse: sse}
}

// WithOrderRepo sets the PG order repository for state persistence.
func (e *OrderExecutor) WithOrderRepo(r *repo.OrderRepo) *OrderExecutor {
	e.orderRepo = r
	return e
}

// WithRiskEventWriter sets the risk event writer for audit persistence.
func (e *OrderExecutor) WithRiskEventWriter(w RiskEventWriter) *OrderExecutor {
	e.riskEventRepo = w
	return e
}

// Submit runs the full state machine: NEW → VALIDATED → RISK_APPROVED → SUBMITTED.
// If orderRepo is nil, falls back to the legacy path (no PG persistence).
func (e *OrderExecutor) Submit(ctx context.Context, req *pb.OrderRequest) (*BrokerResp, error) {
	nowMs := time.Now().UnixMilli()

	// 1. Insert order with state=NEW into PG
	if e.orderRepo != nil {
		order := e.buildOrder(req, pb.OrderState_ORDER_STATE_NEW, "", nowMs)
		if err := e.orderRepo.Insert(ctx, order); err != nil {
			return nil, fmt.Errorf("oms: insert order: %w", err)
		}
		// 2. Transition NEW → VALIDATED
		if err := e.transitionAndPersist(ctx, order, pb.OrderState_ORDER_STATE_VALIDATED, 0); err != nil {
			return nil, fmt.Errorf("oms: validate: %w", err)
		}
	}

	// 3. Risk check
	result := e.risk.Check(ctx, req)
	if !result.Approved {
		if e.orderRepo != nil {
			_ = e.transitionAndPersist(ctx, e.buildOrder(req, pb.OrderState_ORDER_STATE_REJECTED, "", nowMs), pb.OrderState_ORDER_STATE_REJECTED, 0)
		}
		// Persist risk event for audit trail and promotion gate
		if e.riskEventRepo != nil {
			severity := riskSeverity(result.RuleId)
			_ = e.riskEventRepo.Write(ctx, req.TenantId, req.AccountId, req.StrategyId,
				result.RuleId, result.Reason, severity, req)
		}
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

	if e.orderRepo != nil {
		// 4. Transition VALIDATED → RISK_APPROVED
		order := e.buildOrder(req, pb.OrderState_ORDER_STATE_VALIDATED, "", nowMs)
		_ = e.transitionAndPersist(ctx, order, pb.OrderState_ORDER_STATE_RISK_APPROVED, 0)
	}

	// 5. Submit to broker
	resp, err := e.adapter.Submit(ctx, req)
	if err != nil {
		if e.orderRepo != nil {
			order := e.buildOrder(req, pb.OrderState_ORDER_STATE_RISK_APPROVED, "", nowMs)
			_ = e.transitionAndPersist(ctx, order, pb.OrderState_ORDER_STATE_FAILED, 0)
		}
		e.broadcast("order_error", map[string]any{
			"symbol": req.Symbol,
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("oms submit: %w", err)
	}

	if e.orderRepo != nil {
		// 6. Transition RISK_APPROVED → SUBMITTED with broker ticket
		order := e.buildOrder(req, pb.OrderState_ORDER_STATE_RISK_APPROVED, resp.Ticket, nowMs)
		_ = e.transitionAndPersist(ctx, order, pb.OrderState_ORDER_STATE_SUBMITTED, 0)
	}

	// 7. Broadcast SSE event
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

// buildOrder creates an Order proto from a request.
func (e *OrderExecutor) buildOrder(req *pb.OrderRequest, state pb.OrderState, brokerTicket string, tsMs int64) *pb.Order {
	orderID := uuid.NewString()
	clientOrderID := req.ClientOrderId
	if clientOrderID == "" {
		clientOrderID = orderID
	}
	return &pb.Order{
		OrderId:       orderID,
		TenantId:      req.TenantId,
		AccountId:     req.AccountId,
		StrategyId:    req.StrategyId,
		ClientOrderId: clientOrderID,
		BrokerTicket:  brokerTicket,
		Symbol:        req.Symbol,
		Side:          req.Side,
		Type:          req.Type,
		State:         state,
		Price:         req.Price,
		StopPrice:     req.StopPrice,
		Qty:           req.Qty,
		CreatedTsMs:   tsMs,
		UpdatedTsMs:   tsMs,
	}
}

// transitionAndPersist validates the state transition and updates PG.
func (e *OrderExecutor) transitionAndPersist(ctx context.Context, order *pb.Order, next pb.OrderState, filledQty float64) error {
	if err := Transition(order.State, next); err != nil {
		return err
	}
	if e.orderRepo == nil {
		return nil
	}
	return e.orderRepo.UpdateState(ctx, order.OrderId, next, filledQty)
}

// riskSeverity maps a risk rule_id to severity level for audit classification.
// P0: capital-at-risk rules (daily_loss, drawdown, margin)
// P1: operational limits (max_lot, max_position, whitelist)
// P2: all others (session, heartbeart, slippage, reject_rate)
func riskSeverity(ruleID string) string {
	switch ruleID {
	case "daily_loss", "drawdown", "margin":
		return "P0"
	case "max_lot", "max_position", "whitelist":
		return "P1"
	default:
		return "P2"
	}
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
