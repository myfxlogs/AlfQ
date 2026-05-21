package oms

import (
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func TestMT4AdapterConstructor(t *testing.T) {
	a := NewMT4Adapter("mt4grpc3.mtapi.io:443", "user", "pass", "srv")
	if a == nil {
		t.Fatal("NewMT4Adapter returned nil")
	}
	if a.gatewayAddr != "mt4grpc3.mtapi.io:443" {
		t.Errorf("gateway: got %q", a.gatewayAddr)
	}
}

func TestMT5AdapterConstructor(t *testing.T) {
	a := NewMT5Adapter("mt5grpc3.mtapi.io:443", "user", "pass", "srv")
	if a == nil {
		t.Fatal("NewMT5Adapter returned nil")
	}
	if a.gatewayAddr != "mt5grpc3.mtapi.io:443" {
		t.Errorf("gateway: got %q", a.gatewayAddr)
	}
}

func TestMT4AdapterCancel(t *testing.T) {
	a := NewMT4Adapter("x", "user", "pass", "srv")
	if err := a.Cancel(nil, "ticket"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
}

func TestMT4AdapterModify(t *testing.T) {
	a := NewMT4Adapter("x", "user", "pass", "srv")
	if err := a.Modify(nil, "ticket", 1.0, 1.0); err != nil {
		t.Fatalf("Modify: %v", err)
	}
}

func TestMT4AdapterQuery(t *testing.T) {
	a := NewMT4Adapter("x", "user", "pass", "srv")
	order, err := a.Query(nil, "ticket")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if order != nil {
		t.Fatal("expected nil order")
	}
}

func TestBrokerRespFields(t *testing.T) {
	br := &BrokerResp{
		Ticket:    "123",
		State:     pb.OrderState_ORDER_STATE_SUBMITTED,
		FilledQty: 0.1,
		FillPrice: 1.12345,
		ErrorCode: 0,
		ErrorMsg:  "",
	}
	if br.Ticket != "123" {
		t.Errorf("ticket: got %q", br.Ticket)
	}
	if br.State != pb.OrderState_ORDER_STATE_SUBMITTED {
		t.Errorf("state: got %v", br.State)
	}
}

// Interface compliance check at compile time.
func TestInterfaceCompliance(t *testing.T) {
	var _ BrokerAdapter = NewMT4Adapter("x", "", "", "")
	var _ BrokerAdapter = NewMT5Adapter("x", "", "", "")
}
