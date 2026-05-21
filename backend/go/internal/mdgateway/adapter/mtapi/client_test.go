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
