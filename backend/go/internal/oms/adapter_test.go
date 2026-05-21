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

func TestMT5AdapterCancel(t *testing.T) {
	a := NewMT5Adapter("x", "user", "pass", "srv")
	if err := a.Cancel(nil, "ticket"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
}

func TestMT5AdapterModify(t *testing.T) {
	a := NewMT5Adapter("x", "user", "pass", "srv")
	if err := a.Modify(nil, "ticket", 1.0, 1.0); err != nil {
		t.Fatalf("Modify: %v", err)
	}
}

func TestMT5AdapterQuery(t *testing.T) {
	a := NewMT5Adapter("test", "123", "pass", "server")
	order, err := a.Query(nil, "ticket")
	if err != nil {
		t.Fatalf("Query should return nil error, got %v", err)
	}
	if order != nil {
		t.Fatal("Query should return nil order")
	}
}

func TestParseMoney_Nil(t *testing.T) {
	v := parseMoney(nil)
	if v != 0 {
		t.Fatalf("expected 0, got %f", v)
	}
}

func TestSplitHostPort(t *testing.T) {
	host, port := splitHostPort("example.com:443", "80")
	if host != "example.com" {
		t.Fatalf("expected example.com, got %s", host)
	}
	if port != "443" {
		t.Fatalf("expected 443, got %s", port)
	}
}

func TestSplitHostPort_NoPort(t *testing.T) {
	host, port := splitHostPort("example.com", "80")
	if host != "example.com" {
		t.Fatalf("expected example.com, got %s", host)
	}
	if port != "80" {
		t.Fatalf("expected 80, got %s", port)
	}
}

func TestParseUint(t *testing.T) {
	n := parseUint("12345")
	if n != 12345 {
		t.Fatalf("expected 12345, got %d", n)
	}
}

func TestParseUint_Empty(t *testing.T) {
	n := parseUint("")
	if n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestParsePort(t *testing.T) {
	n := parsePort("443")
	if n != 443 {
		t.Fatalf("expected 443, got %d", n)
	}
}

func TestParsePort_Empty(t *testing.T) {
	n := parsePort("")
	if n != 443 {
		t.Fatalf("expected 443, got %d", n)
	}
}

func TestPtrUint64(t *testing.T) {
	p := ptrUint64(123)
	if p == nil {
		t.Fatal("ptrUint64 returned nil")
	}
	if *p != 123 {
		t.Fatalf("expected 123, got %d", *p)
	}
}

func TestPtrString(t *testing.T) {
	p := ptrString("test")
	if p == nil {
		t.Fatal("ptrString returned nil")
	}
	if *p != "test" {
		t.Fatalf("expected test, got %s", *p)
	}
}
