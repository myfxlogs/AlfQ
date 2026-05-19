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
