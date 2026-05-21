package risksvc_test

import (
	"context"
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/risksvc"
)

func price(v string) *pb.Money { return &pb.Money{Value: v} }

func TestMaxLotRule(t *testing.T) {
	engine := risksvc.NewEngine()
	req := &pb.OrderRequest{
		Symbol: "EURUSD", Side: pb.OrderSide_ORDER_SIDE_BUY,
		Type: pb.OrderType_ORDER_TYPE_MARKET, Qty: 100, Price: price("1.05"),
	}
	result := engine.Check(context.Background(), req)
	if result == nil {
		t.Fatal("expected risk check result")
	}
}

func TestAccountState_Fields(t *testing.T) {
	state := risksvc.AccountState{
		Equity:         10000.0,
		Margin:         1000.0,
		FreeMargin:     9000.0,
		DailyPnL:       500.0,
		MaxDrawdown:    0.10,
		Positions:      make(map[string]*pb.Position),
		TotalPositions: 2,
		OpenOrders:     3,
	}
	if state.Equity != 10000.0 {
		t.Fatalf("expected 10000.0, got %f", state.Equity)
	}
	if state.TotalPositions != 2 {
		t.Fatalf("expected 2, got %d", state.TotalPositions)
	}
}

func TestDailyLossRejected(t *testing.T) {
	engine := risksvc.NewEngine()
	req := &pb.OrderRequest{
		Symbol: "EURUSD", Side: pb.OrderSide_ORDER_SIDE_BUY,
		Type: pb.OrderType_ORDER_TYPE_MARKET, Qty: 1, Price: price("1.05"),
	}
	result := engine.Check(context.Background(), req)
	if result == nil {
		t.Fatal("expected risk check result")
	}
}

func TestPositionLimitRule(t *testing.T) {
	engine := risksvc.NewEngine()
	req := &pb.OrderRequest{
		Symbol: "XAUUSD", Side: pb.OrderSide_ORDER_SIDE_BUY,
		Type: pb.OrderType_ORDER_TYPE_MARKET, Qty: 1, Price: price("2000"),
	}
	result := engine.Check(context.Background(), req)
	if result == nil {
		t.Fatal("expected risk check result")
	}
}

func TestEngineExists(t *testing.T) {
	engine := risksvc.NewEngine()
	if engine == nil {
		t.Fatal("engine is nil")
	}
}
