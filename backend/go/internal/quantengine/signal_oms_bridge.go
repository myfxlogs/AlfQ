// Package quantengine — signal-to-OMS bridge.
//
// Bridges strategy signals (from DSL or ONNX) to OMS order submission
// with canonical→symbol_raw resolution, risk checks, and SSE broadcast.
package quantengine

import (
	"context"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/oms"
	"go.uber.org/zap"
)

// SignalToOMS creates a SignalHandler that converts signals to OMS orders.
//
// The returned handler performs:
//  1. Lookup broker symbol_raw from canonical (via symbolResolver)
//  2. Build OrderRequest
//  3. Submit via OrderExecutor (risk check + broker submit + SSE broadcast)
func SignalToOMS(
	executor *oms.OrderExecutor,
	accountID string,
	resolveSymbol func(canonical string) (string, error),
	log *zap.Logger,
) SignalHandler {
	return func(canonical string, side string, qty float64, reason string) {
		// Resolve canonical → broker symbol_raw
		symbolRaw, err := resolveSymbol(canonical)
		if err != nil {
			log.Warn("symbol resolution failed",
				zap.String("canonical", canonical),
				zap.Error(err),
			)
			return
		}

		// Build order request
		var orderSide pb.OrderSide
		switch side {
		case "long", "buy":
			orderSide = pb.OrderSide_ORDER_SIDE_BUY
		case "short", "sell":
			orderSide = pb.OrderSide_ORDER_SIDE_SELL
		default:
			log.Warn("unknown signal direction", zap.String("side", side))
			return
		}

		req := &pb.OrderRequest{
			AccountId:  accountID,
			StrategyId: reason, // strategy name
			Symbol:     symbolRaw,
			Side:       orderSide,
			Type:       pb.OrderType_ORDER_TYPE_MARKET,
			Qty:        qty,
		}

		// Submit via executor (risk check + broker + SSE)
		resp, err := executor.Submit(context.Background(), req)
		if err != nil {
			log.Warn("order submit failed",
				zap.String("canonical", canonical),
				zap.String("symbol_raw", symbolRaw),
				zap.String("side", side),
				zap.Error(err),
			)
			return
		}

		log.Info("order submitted",
			zap.String("ticket", resp.Ticket),
			zap.String("canonical", canonical),
			zap.String("symbol_raw", symbolRaw),
			zap.String("side", side),
			zap.Float64("qty", qty),
			zap.String("state", resp.State.String()),
		)
	}
}

// DefaultSymbolResolver returns a resolver that uses the canonical name as-is
// (fallback when broker_symbols table is not available).
func DefaultSymbolResolver() func(canonical string) (string, error) {
	return func(canonical string) (string, error) {
		return canonical, nil
	}
}

// PGSymbolResolver creates a symbol resolver from the broker_symbols PG table.
func PGSymbolResolver(brokerID string, queryRow func(ctx context.Context, query string, args ...any) (string, error)) func(canonical string) (string, error) {
	return func(canonical string) (string, error) {
		return queryRow(context.Background(),
			`SELECT symbol_raw FROM broker_symbols WHERE broker_id = $1 AND canonical = $2 LIMIT 1`,
			brokerID, canonical,
		)
	}
}


