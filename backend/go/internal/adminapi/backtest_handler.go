// Package adminapi — BacktestService handler implementation (LP-1).
//
// Flow:
//
//	RunBacktest → load strategy → call Python research CLI →
//	vectorized + event backtest → consistency gate →
//	update strategy status (draft→ready) → stream progress.
package adminapi

import (
	"context"
	"encoding/json"
	"time"

	"connectrpc.com/connect"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/backtest"
)

// RunBacktest executes a full backtest for a strategy and streams progress.
func (a *Adapter) RunBacktest(
	ctx context.Context,
	req *connect.Request[pb.RunBacktestRequest],
	stream *connect.ServerStream[pb.BacktestProgress],
) error {
	strategyID := req.Msg.StrategyId
	if strategyID == "" {
		return connect.NewError(connect.CodeInvalidArgument, nil)
	}

	// 1. Load strategy from PG
	strategy, err := a.svc.GetStrategy(ctx, &pb.GetStrategyRequest{Id: strategyID})
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}

	sendProgress(stream, strategyID, "loading", "{}", 0)

	// 2. Parse spec JSON
	var specObj struct {
		Name             string            `json:"name"`
		CanonicalSymbols []string          `json:"canonical_symbols"`
		Period           string            `json:"period"`
		Factors          map[string]string `json:"factors"`
		SignalRule       string            `json:"signal_rule"`
	}
	if err := json.Unmarshal([]byte(strategy.SpecJson), &specObj); err != nil {
		sendProgress(stream, strategyID, "error", `{"error":"invalid spec json"}`, 0)
		return nil
	}

	sendProgress(stream, strategyID, "running", "{}", 10)

	// 3. Run Python backtest CLI
	result, err := backtest.RunPythonBacktest(ctx, []byte(strategy.SpecJson), "research", 5*time.Minute)
	if err != nil {
		sendProgress(stream, strategyID, "error", `{"error":"backtest failed"}`, 0)
		return nil
	}

	sendProgress(stream, strategyID, "running", "{}", 80)

	// 4. Consistency gate
	passed := result.Status == "passed" && result.Correlation >= 0.95 && result.DailyMADPct < 0.01

	// 5. Update strategy status
	newStatus := strategy.Status
	if passed {
		newStatus = "ready"
	}

	if err := a.svc.updateStrategyStatus(ctx, strategyID, newStatus); err != nil {
		sendProgress(stream, strategyID, "warning", `{"error":"status update failed"}`, 95)
	} else {
		sendProgress(stream, strategyID, "running", "{}", 95)
	}

	// 6. Send final result with JSON payload
	resultJSON, _ := json.Marshal(map[string]any{
		"passed":      passed,
		"correlation": result.Correlation,
		"daily_mad":   result.DailyMADPct,
		"vec_sharpe":  result.VecSharpe,
		"ev_sharpe":   result.EvSharpe,
		"new_status":  newStatus,
	})
	finalStatus := "completed"
	if !passed {
		finalStatus = "failed"
	}
	sendProgress(stream, strategyID, finalStatus, string(resultJSON), 100)

	return nil
}

// ListBacktests returns historical backtest records.
func (a *Adapter) ListBacktests(
	ctx context.Context,
	req *connect.Request[pb.ListBacktestsRequest],
) (*connect.Response[pb.ListBacktestsResponse], error) {
	resp, err := a.svc.ListBacktests(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func sendProgress(stream *connect.ServerStream[pb.BacktestProgress], taskID, status, resultJSON string, progress float64) {
	_ = stream.Send(&pb.BacktestProgress{
		TaskId:     taskID,
		Status:     status,
		Progress:   progress,
		ResultJson: resultJSON,
	})
}

// ListBacktests returns historical backtest records from the service.
func (s *Service) ListBacktests(ctx context.Context, req *pb.ListBacktestsRequest) (*pb.ListBacktestsResponse, error) {
	_ = req
	return &pb.ListBacktestsResponse{}, nil
}

// updateStrategyStatus updates a strategy's status in PG.
func (s *Service) updateStrategyStatus(ctx context.Context, strategyID, newStatus string) error {
	if s.pool == nil {
		return nil
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE strategies SET status = $1 WHERE id = $2`,
		newStatus, strategyID,
	)
	return err
}
