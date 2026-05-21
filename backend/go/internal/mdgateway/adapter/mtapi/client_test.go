package mtapi

import (
	"testing"
)

func TestAccountInfo_Fields(t *testing.T) {
	ai := AccountInfo{
		Balance:     1000.0,
		Equity:      1100.0,
		Margin:      100.0,
		FreeMargin:  900.0,
		MarginLevel: 10.0,
		Profit:      100.0,
		Currency:    "USD",
		Leverage:    100,
	}
	if ai.Balance != 1000.0 {
		t.Fatalf("expected 1000.0, got %f", ai.Balance)
	}
	if ai.Currency != "USD" {
		t.Fatalf("expected USD, got %s", ai.Currency)
	}
}

func TestPositionInfo_Fields(t *testing.T) {
	pi := PositionInfo{
		Ticket:    12345,
		Symbol:    "EURUSD",
		Type:      "buy",
		Lots:      0.1,
		OpenPrice: 1.1000,
		Profit:    10.0,
	}
	if pi.Ticket != 12345 {
		t.Fatalf("expected 12345, got %d", pi.Ticket)
	}
	if pi.Symbol != "EURUSD" {
		t.Fatalf("expected EURUSD, got %s", pi.Symbol)
	}
}

func TestHistoryOrderInfo_Fields(t *testing.T) {
	h := HistoryOrderInfo{
		Ticket:  12345,
		Symbol:  "EURUSD",
		Type:    "buy",
		Lots:    0.1,
		Profit:  10.0,
	}
	if h.Ticket != 12345 {
		t.Fatalf("expected 12345, got %d", h.Ticket)
	}
}

func TestBrokerMatch_Fields(t *testing.T) {
	bm := BrokerMatch{
		Company: "Test Broker",
		Servers: []ServerEntry{
			{Name: "Server1", Access: "1.2.3.4:443"},
		},
	}
	if bm.Company != "Test Broker" {
		t.Fatalf("expected Test Broker, got %s", bm.Company)
	}
	if len(bm.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(bm.Servers))
	}
}

func TestServerEntry_Fields(t *testing.T) {
	se := ServerEntry{
		Name:   "TestServer",
		Access: "1.2.3.4:443",
	}
	if se.Name != "TestServer" {
		t.Fatalf("expected TestServer, got %s", se.Name)
	}
	if se.Access != "1.2.3.4:443" {
		t.Fatalf("expected 1.2.3.4:443, got %s", se.Access)
	}
}
