package oms

import (
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func TestNewOrderExecutor(t *testing.T) {
	e := NewOrderExecutor(nil, nil, nil)
	if e == nil {
		t.Fatal("NewOrderExecutor returned nil")
	}
}

func TestBrokerResp_Fields(t *testing.T) {
	resp := &BrokerResp{
		Ticket:    "ticket-123",
		State:     pb.OrderState_ORDER_STATE_SUBMITTED,
		FilledQty: 0.1,
		FillPrice: 1.1000,
		ErrorCode: 0,
		ErrorMsg:  "",
	}
	if resp.Ticket != "ticket-123" {
		t.Fatalf("expected ticket-123, got %s", resp.Ticket)
	}
	if resp.State != pb.OrderState_ORDER_STATE_SUBMITTED {
		t.Fatalf("expected SUBMITTED, got %s", resp.State)
	}
}
