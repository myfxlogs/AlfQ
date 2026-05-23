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

	"github.com/alfq/backend/go/internal/adminapi"
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

	mthubAddr := os.Getenv("MTHUB_ADDR")
	if mthubAddr == "" {
		mthubAddr = "md-gateway:9001"
	}

	// ── OMS wiring ──
	orderRepo := repo.NewOrderRepo(d.PG)
	riskEventRepo := repo.NewRiskEventRepo(d.PG)
	riskEngine := risksvc.NewEngine()
	if d.PG != nil {
		riskEngine.WithCanonicalAuth(d.PG.Pool)
		d.Log.Info("canonical auth rule activated (Gate 1+2)")
	}

	mthub := newMthubAdapter(mthubAddr, d.Log)

	executor := oms.NewOrderExecutor(mthub, riskEngine, nil).
		WithOrderRepo(orderRepo).
		WithRiskEventWriter(riskEventRepo)

	// Wire canonical → symbol_raw resolver (Gate-3 in OMS executor)
	if d.PG != nil {
		resolver := adminapi.NewSymbolResolver(d.PG.Pool)
		executor.WithSymbolResolver(&omsSymbolResolver{resolver: resolver})
		d.Log.Info("symbol resolver wired for canonical resolution")
	}

	d.Log.Info("oms executor wired for quant-engine",
		zap.String("account", accountID),
		zap.String("mthub", mthubAddr),
	)

	// Wire signal→order bridge: signals flow through OMS state machine
	// Symbol resolution (canonical → broker_symbol_raw) is handled by OrderExecutor.
	onSignal := func(strategyID, symbol, side string, qty float64, reason string) {
		if strategyID == "" {
			d.Log.Warn("signal dropped: missing strategy_id", zap.String("reason", reason))
			return
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

		// Symbol = canonical; OMS executor resolves BrokerSymbolRaw via SymbolResolver
		req := &pb.OrderRequest{
			TenantId:   tenantID,
			AccountId:  accountID,
			StrategyId: strategyID,
			Symbol:     symbol,
			Side:       orderSide,
			Qty:        qty,
			Type:       pb.OrderType_ORDER_TYPE_MARKET,
			ClientOrderId: fmt.Sprintf("qe-%s-%d", symbol, time.Now().UTC().UnixMilli()),
		}

		resp, err := executor.Submit(context.Background(), req)
		if err != nil {
			d.Log.Warn("order submit failed (oms)",
				zap.String("symbol", symbol),
				zap.String("side", side),
				zap.Error(err),
			)
			return
		}
		d.Log.Info("order submitted via oms",
			zap.String("canonical", symbol),
			zap.String("broker_symbol", req.BrokerSymbolRaw),
			zap.String("side", side),
			zap.Float64("qty", qty),
			zap.String("ticket", resp.Ticket),
		)
	}

	return quantengine.RunQuantEngineWithSignalHandler(adapter.Mux, d, onSignal)
}

// omsSymbolResolver adapts adminapi.SymbolResolver to oms.SymbolResolver.
type omsSymbolResolver struct {
	resolver *adminapi.SymbolResolver
}

func (r *omsSymbolResolver) ResolveCanonical(ctx context.Context, accountID, canonical string) (string, int32, error) {
	info, valid, err := r.resolver.ResolveCanonical(ctx, accountID, canonical)
	if err != nil {
		return "", 0, err
	}
	if !valid {
		return info.SymbolRaw, info.TradeMode, nil // trade_mode=0 → disabled
	}
	return info.SymbolRaw, info.TradeMode, nil
}
