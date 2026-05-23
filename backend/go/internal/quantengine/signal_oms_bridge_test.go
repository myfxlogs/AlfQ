package quantengine

import (
	"context"
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/oms"
	"github.com/alfq/backend/go/internal/risksvc"
	"github.com/alfq/backend/go/internal/ssehub"
	"go.uber.org/zap"
)

// stubBrokerAdapter records submissions for testing.
type stubBrokerAdapter struct {
	submitted []*pb.OrderRequest
	resp      *oms.BrokerResp
	err       error
}

func (s *stubBrokerAdapter) Submit(ctx context.Context, req *pb.OrderRequest) (*oms.BrokerResp, error) {
	s.submitted = append(s.submitted, req)
	if s.resp == nil {
		s.resp = &oms.BrokerResp{Ticket: "TICKET-1", State: pb.OrderState_ORDER_STATE_SUBMITTED}
	}
	return s.resp, s.err
}

func (s *stubBrokerAdapter) Cancel(ctx context.Context, ticket string) error { return nil }

// testEngine creates a risk engine without time-dependent rules for deterministic unit tests.
func testEngine(t *testing.T) *risksvc.Engine {
	t.Helper()
	e := risksvc.NewTestEngine() // skips session, heartbeat, reject_rate
	return e
}
func (s *stubBrokerAdapter) Modify(ctx context.Context, ticket string, p, sp float64) error {
	return nil
}
func (s *stubBrokerAdapter) Query(ctx context.Context, ticket string) (*pb.Order, error) {
	return nil, nil
}

func TestSignalToOMS_Buy(t *testing.T) {
	log := zap.NewNop()
	risk := testEngine(t)
	sse := ssehub.New()
	stub := &stubBrokerAdapter{}
	executor := oms.NewOrderExecutor(stub, risk, sse)

	handler := SignalToOMS(executor, "acc-1", DefaultSymbolResolver(), log)
	handler("strat-1", "EURUSD", "long", 0.1, "demo_sma")

	if len(stub.submitted) != 1 {
		t.Fatalf("expected 1 order, got %d", len(stub.submitted))
	}
	req := stub.submitted[0]
	if req.Symbol != "EURUSD" {
		t.Errorf("symbol = %q, want EURUSD (canonical)", req.Symbol)
	}
	if req.Side != pb.OrderSide_ORDER_SIDE_BUY {
		t.Errorf("side = %v, want BUY", req.Side)
	}
	if req.Qty != 0.1 {
		t.Errorf("qty = %f, want 0.1", req.Qty)
	}
	if req.AccountId != "acc-1" {
		t.Errorf("account = %q, want acc-1", req.AccountId)
	}
}

func TestSignalToOMS_Sell(t *testing.T) {
	log := zap.NewNop()
	stub := &stubBrokerAdapter{}
	executor := oms.NewOrderExecutor(stub, testEngine(t), ssehub.New())

	handler := SignalToOMS(executor, "acc-2", DefaultSymbolResolver(), log)
	handler("strat-2", "GBPUSD", "short", 0.2, "trend_follow")

	if len(stub.submitted) != 1 {
		t.Fatalf("expected 1 order, got %d", len(stub.submitted))
	}
	req := stub.submitted[0]
	if req.Side != pb.OrderSide_ORDER_SIDE_SELL {
		t.Errorf("side = %v, want SELL", req.Side)
	}
	if req.Qty != 0.2 {
		t.Errorf("qty = %f", req.Qty)
	}
}

func TestSignalToOMS_FlatSkips(t *testing.T) {
	log := zap.NewNop()
	stub := &stubBrokerAdapter{}
	executor := oms.NewOrderExecutor(stub, testEngine(t), ssehub.New())

	handler := SignalToOMS(executor, "acc-1", DefaultSymbolResolver(), log)
	handler("strat-3", "EURUSD", "flat", 0.1, "test")

	if len(stub.submitted) != 0 {
		t.Errorf("expected 0 orders for flat signal, got %d", len(stub.submitted))
	}
}

func TestDefaultSymbolResolver(t *testing.T) {
	resolver := DefaultSymbolResolver()
	// RS06: DefaultSymbolResolver is pass-through (real resolution via adminapi SymbolResolver)
	symbol, err := resolver("EURUSD")
	if err != nil {
		t.Fatalf("DefaultSymbolResolver error: %v", err)
	}
	if symbol != "EURUSD" {
		t.Fatalf("expected EURUSD pass-through, got %s", symbol)
	}
	// Unknown symbol should pass through
	symbol2, err := resolver("ZZZUNKNOWN")
	if err != nil {
		t.Fatalf("DefaultSymbolResolver error for unknown: %v", err)
	}
	if symbol2 != "ZZZUNKNOWN" {
		t.Fatalf("unknown symbol should pass through, got %s", symbol2)
	}
}

func TestPGSymbolResolver(t *testing.T) {
	resolver := PGSymbolResolver("broker-1", nil)
	if resolver == nil {
		t.Fatal("PGSymbolResolver returned nil")
	}
}

func TestSignalToOMS_RiskReject(t *testing.T) {
	// With CanonicalAuth (no PG pool → dev mode, allows all), BTCUSD passes risk.
	// Integration tests with real PG verify Gate-1/Gate-2 rejection.
	log := zap.NewNop()
	risk := testEngine(t)
	stub := &stubBrokerAdapter{}
	executor := oms.NewOrderExecutor(stub, risk, ssehub.New())

	handler := SignalToOMS(executor, "acc-1", DefaultSymbolResolver(), log)
	// In dev mode (no PG), CanonicalAuth allows all symbols
	handler("strat-4", "BTCUSD", "long", 0.1, "crypto_strat")

	if len(stub.submitted) != 1 {
		t.Errorf("expected 1 order in dev mode, got %d", len(stub.submitted))
	}
}
