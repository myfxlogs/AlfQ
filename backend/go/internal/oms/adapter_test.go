package oms

import (
	"context"
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func TestNewMT4Adapter(t *testing.T) {
	a := NewMT4Adapter("user", "pass", "srv")
	if a == nil {
		t.Fatal("NewMT4Adapter returned nil")
	}
}

func TestNewMT5Adapter(t *testing.T) {
	a := NewMT5Adapter("user", "pass", "srv")
	if a == nil {
		t.Fatal("NewMT5Adapter returned nil")
	}
}

func TestMT4AdapterSubmit(t *testing.T) {
	a := NewMT4Adapter("user", "pass", "srv")
	resp, err := a.Submit(context.Background(), &pb.OrderRequest{})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if resp.State != pb.OrderState_ORDER_STATE_SUBMITTED {
		t.Fatalf("State: got %v", resp.State)
	}
}

func TestMT4AdapterCancel(t *testing.T) {
	a := NewMT4Adapter("user", "pass", "srv")
	if err := a.Cancel(context.Background(), "ticket"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
}

func TestMT4AdapterModify(t *testing.T) {
	a := NewMT4Adapter("user", "pass", "srv")
	if err := a.Modify(context.Background(), "ticket", 1.0, 1.0); err != nil {
		t.Fatalf("Modify: %v", err)
	}
}

func TestMT4AdapterQuery(t *testing.T) {
	a := NewMT4Adapter("user", "pass", "srv")
	order, err := a.Query(context.Background(), "ticket")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if order != nil {
		t.Fatal("expected nil order")
	}
}

func TestMT5AdapterMethods(t *testing.T) {
	a := NewMT5Adapter("user", "pass", "srv")
	resp, err := a.Submit(context.Background(), &pb.OrderRequest{})
	if err != nil || resp.State != pb.OrderState_ORDER_STATE_SUBMITTED {
		t.Fatal("MT5 Submit failed")
	}
	if err := a.Cancel(context.Background(), "t"); err != nil {
		t.Fatal("MT5 Cancel failed")
	}
	if err := a.Modify(context.Background(), "t", 1, 1); err != nil {
		t.Fatal("MT5 Modify failed")
	}
	if _, err := a.Query(context.Background(), "t"); err != nil {
		t.Fatal("MT5 Query failed")
	}
}
