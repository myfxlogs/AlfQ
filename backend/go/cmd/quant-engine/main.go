// quant-engine merges factor-svc and strategy-svc into a single process.
// Signal → OMS bridge: signals flow through oms.OrderExecutor.Submit
// for risk checking, PG persistence, and broker submission.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/oms"
	"github.com/alfq/backend/go/internal/oms/repo"
	"github.com/alfq/backend/go/internal/quantengine"
	"github.com/alfq/backend/go/internal/risksvc"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"go.uber.org/zap"
)

func main() {
	if err := bootstrap.Run("quant-engine", register,
		bootstrap.WithoutRedis(),
	); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
		os.Exit(1)
	}
}

// mthubAdapter implements oms.BrokerAdapter by calling md-gateway's mthub HTTP endpoint.
type mthubAdapter struct {
	baseURL    string
	httpClient *http.Client
	log        *zap.Logger
}

func newMthubAdapter(mthubAddr string, log *zap.Logger) *mthubAdapter {
	return &mthubAdapter{
		baseURL:    fmt.Sprintf("http://%s", mthubAddr),
		httpClient: &http.Client{Timeout: 15 * time.Second},
		log:        log,
	}
}

func (a *mthubAdapter) Submit(ctx context.Context, req *pb.OrderRequest) (*oms.BrokerResp, error) {
	mtSide := "buy"
	if req.Side == pb.OrderSide_ORDER_SIDE_SELL {
		mtSide = "sell"
	}

	reqBody := map[string]interface{}{
		"account_id": req.AccountId,
		"symbol":     req.Symbol,
		"side":       mtSide,
		"lots":       req.Qty,
		"comment":    "qe-auto",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	resp, err := a.httpClient.Post(
		a.baseURL+"/alfq.mthub.v1.MtHubService/OrderSend",
		"application/json",
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("mthub submit: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("mthub returned %d: %s", resp.StatusCode, string(respBody))
	}

	var mthubResp struct {
		Ticket json.Number `json:"ticket"`
		Error  string      `json:"error"`
	}
	if err := json.Unmarshal(respBody, &mthubResp); err != nil {
		return nil, fmt.Errorf("mthub parse error: %w", err)
	}
	if mthubResp.Error != "" {
		return nil, fmt.Errorf("mthub error: %s", mthubResp.Error)
	}

	ticket, _ := mthubResp.Ticket.Int64()
	if ticket <= 0 {
		return nil, fmt.Errorf("mthub no ticket in response")
	}

	return &oms.BrokerResp{Ticket: fmt.Sprintf("%d", ticket)}, nil
}

func (a *mthubAdapter) Cancel(ctx context.Context, ticket string) error {
	a.log.Warn("mthub Cancel not implemented", zap.String("ticket", ticket))
	return nil
}

func (a *mthubAdapter) Modify(ctx context.Context, ticket string, price, stopPrice float64) error {
	a.log.Warn("mthub Modify not implemented", zap.String("ticket", ticket))
	return nil
}

func (a *mthubAdapter) Query(ctx context.Context, ticket string) (*pb.Order, error) {
	a.log.Warn("mthub Query not implemented", zap.String("ticket", ticket))
	return nil, nil
}

func register(adapter *bootstrap.ServeMuxAdapter, d *bootstrap.Deps) error {
	accountID := "51b8fe22-1561-4027-802d-32af80d17f6d" // MT5 demo
	tenantID := "00000000-0000-0000-0000-000000000001"
	strategyID := "70846f7a-9873-492f-82f7-7ac48d26551e"

	mthubAddr := os.Getenv("MTHUB_ADDR")
	if mthubAddr == "" {
		mthubAddr = "md-gateway:9001"
	}

	// ── OMS wiring ──
	orderRepo := repo.NewOrderRepo(d.PG)
	riskEventRepo := repo.NewRiskEventRepo(d.PG)
	riskEngine := risksvc.NewEngine()

	mthub := newMthubAdapter(mthubAddr, d.Log)

	executor := oms.NewOrderExecutor(mthub, riskEngine, nil).
		WithOrderRepo(orderRepo).
		WithRiskEventWriter(riskEventRepo)

	d.Log.Info("oms executor wired for quant-engine",
		zap.String("account", accountID),
		zap.String("mthub", mthubAddr),
	)

	// Wire signal→order bridge: signals flow through OMS state machine
	onSignal := func(symbol string, side string, qty float64, reason string) {
		// Resolve canonical → broker-specific symbol_raw
		brokerSymbol := symbol
		if d.PG != nil {
			var resolved string
			if err := d.PG.QueryRow(context.Background(),
				`SELECT bs.symbol_raw
				   FROM accounts a
				   JOIN broker_symbols bs ON bs.broker_id = a.broker_id
				  WHERE a.id = $1
				    AND (bs.canonical = $2 OR bs.symbol_raw = $2)
				  LIMIT 1`,
				accountID, symbol,
			).Scan(&resolved); err == nil && resolved != "" {
				brokerSymbol = resolved
			}
		}

		var orderSide pb.OrderSide
		switch side {
		case "long", "buy":
			orderSide = pb.OrderSide_ORDER_SIDE_BUY
		case "short", "sell":
			orderSide = pb.OrderSide_ORDER_SIDE_SELL
		default:
			d.Log.Warn("unknown signal direction", zap.String("side", side))
			return
		}

		req := &pb.OrderRequest{
			TenantId:   tenantID,
			AccountId:  accountID,
			StrategyId: strategyID,
			Symbol:     brokerSymbol,
			Side:       orderSide,
			Qty:        qty,
			Type:       pb.OrderType_ORDER_TYPE_MARKET,
			ClientOrderId: fmt.Sprintf("qe-%s-%d", brokerSymbol, time.Now().UTC().UnixMilli()),
		}

		resp, err := executor.Submit(context.Background(), req)
		if err != nil {
			d.Log.Warn("order submit failed (oms)",
				zap.String("symbol", brokerSymbol),
				zap.String("side", side),
				zap.Error(err),
			)
			return
		}
		d.Log.Info("order submitted via oms",
			zap.String("canonical", symbol),
			zap.String("broker_symbol", brokerSymbol),
			zap.String("side", side),
			zap.Float64("qty", qty),
			zap.String("ticket", resp.Ticket),
		)
	}

	return quantengine.RunQuantEngineWithSignalHandler(adapter.Mux, d, onSignal)
}
