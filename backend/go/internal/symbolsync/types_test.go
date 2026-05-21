// Package symbolsync — type conversion and session formatting tests.
package symbolsync

import (
	"testing"
)

func TestBrokerSymbol(t *testing.T) {
	bs := &BrokerSymbol{
		BrokerID:  "test",
		SymbolRaw: "EURUSD",
		Canonical:  "EURUSD",
	}
	if bs.BrokerID != "test" {
		t.Fatalf("expected test, got %s", bs.BrokerID)
	}
}

func TestBrokerSymbol_Fields(t *testing.T) {
	bs := &BrokerSymbol{
		BrokerID:        "broker1",
		SymbolRaw:       "EURUSD",
		Canonical:       "EURUSD",
		Digits:          5,
		Point:           0.00001,
		TickSize:        0.00001,
		TickValue:       0.1,
		ContractSize:    100000,
		MinLot:          0.01,
		MaxLot:          100,
		LotStep:         0.01,
		MarginInitial:   1000,
		MarginCurrency:  "USD",
		ProfitCurrency:  "USD",
		SwapLong:        -3.5,
		SwapShort:       1.2,
		SwapMode:        1,
		SwapRolloverDay: 3,
		TradeMode:       3,
		Description:     "Euro vs US Dollar",
		SessionsQuote:   []byte(`[{"sessions":[{"start":0,"end":86400}]}]`),
		SessionsTrade:   []byte(`[{"sessions":[{"start":0,"end":86400}]}]`),
		ServerTimezone:  "+2",
		RawPayload:      []byte(`{}`),
		Partial:         false,
	}

	if bs.Digits != 5 {
		t.Fatalf("expected 5, got %d", bs.Digits)
	}
	if bs.ContractSize != 100000 {
		t.Fatalf("expected 100000, got %f", bs.ContractSize)
	}
}

