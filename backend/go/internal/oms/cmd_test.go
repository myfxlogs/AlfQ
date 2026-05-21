//go:build ignore

package main

import (
	"context"
	"fmt"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/oms"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("=== MT5 OrderSend ===")
	a := oms.NewMT5Adapter("mt5grpc3.mtapi.io:443", "277259925", "HavEr7901$", "18.163.85.196:443")
	req := &pb.OrderRequest{Symbol: "EURUSDm", Side: pb.OrderSide_ORDER_SIDE_BUY, Qty: 0.01, StrategyId: "test-d"}
	resp, err := a.Submit(ctx, req)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
	} else {
		fmt.Printf("ticket=%s state=%v filled=%f errCode=%d errMsg=%s\n", resp.Ticket, resp.State, resp.FilledQty, resp.ErrorCode, resp.ErrorMsg)
	}

	fmt.Println("\n=== MT4 OrderSend ===")
	a4 := oms.NewMT4Adapter("mt4grpc3.mtapi.io:443", "95172262", "HavEr7901$", "43.199.125.167:443")
	req4 := &pb.OrderRequest{Symbol: "EURUSDm", Side: pb.OrderSide_ORDER_SIDE_BUY, Qty: 0.01, StrategyId: "test-d"}
	resp4, err := a4.Submit(ctx, req4)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
	} else {
		fmt.Printf("ticket=%s state=%v filled=%f errCode=%d errMsg=%s\n", resp4.Ticket, resp4.State, resp4.FilledQty, resp4.ErrorCode, resp4.ErrorMsg)
	}
}
