// Package symbolsync — convert tests with proto-level mock fixtures.
package symbolsync

import (
	"testing"

	mt5pb "github.com/alfq/backend/go/gen/mt5"
)

func TestConvertMT5SymbolEURUSD(t *testing.T) {
	sp := &mt5pb.SymbolParams{
		Symbol: "EURUSD",
		SymbolInfo: &mt5pb.SymbolInfo{
			Digits:         5,
			Points:         1e-5,
			TickSize:       1e-5,
			TickValue:      1.0,
			ContractSize:   100000,
			Description:    "Euro vs US Dollar",
			MarginCurrency: "USD",
			ProfitCurrency: "USD",
		},
		SymbolGroup: &mt5pb.SymGroup{
			MinLots:       0.01,
			MaxLots:       100,
			LotsStep:      0.01,
			InitialMargin: 1000,
			SwapLong:      -3.5,
			SwapShort:     1.2,
			TradeMode:     3,
			SwapType:      1,
			ThreeDaysSwap: 3,
		},
	}

	tz := &mt5pb.ServerTimezoneReply{
		Result: 2.0,
	}

	sym := ConvertMT5Symbol(sp, "b1", nil, tz)

	if sym.BrokerID != "b1" {
		t.Errorf("broker: got %q, want b1", sym.BrokerID)
	}
	if sym.SymbolRaw != "EURUSD" {
		t.Errorf("raw: got %q, want EURUSD", sym.SymbolRaw)
	}
	if sym.Canonical != "EURUSD" {
		t.Errorf("canonical: got %q, want EURUSD", sym.Canonical)
	}
	if sym.Digits != 5 {
		t.Errorf("digits: got %d, want 5", sym.Digits)
	}
	if sym.ContractSize != 100000 {
		t.Errorf("contract_size: got %f, want 100000", sym.ContractSize)
	}
	if sym.MinLot != 0.01 {
		t.Errorf("min_lot: got %f, want 0.01", sym.MinLot)
	}
	if sym.ServerTimezone != "+2" {
		t.Errorf("tz: got %q, want +2", sym.ServerTimezone)
	}
	if sym.Partial {
		t.Error("full symbol marked partial")
	}
}

func TestConvertMT5SymbolWithSuffix(t *testing.T) {
	sp := &mt5pb.SymbolParams{
		Symbol: "EURUSD.m",
		SymbolInfo: &mt5pb.SymbolInfo{
			Digits:       5,
			Points:       1e-5,
			ContractSize: 100000,
		},
		SymbolGroup: &mt5pb.SymGroup{
			TradeMode: 3,
		},
	}

	sym := ConvertMT5Symbol(sp, "b1", nil, nil)
	if sym.Canonical != "EURUSD" {
		t.Errorf("canonical for EURUSD.m: got %q, want EURUSD", sym.Canonical)
	}
}

func TestConvertMT5SymbolPartial(t *testing.T) {
	// Missing Digits → partial
	sp := &mt5pb.SymbolParams{
		Symbol: "XAUUSD",
		SymbolInfo: &mt5pb.SymbolInfo{
			Digits: 0, // invalid
		},
	}

	sym := ConvertMT5Symbol(sp, "b1", nil, nil)
	if !sym.Partial {
		t.Error("expected partial=true for zero digits")
	}
}

func TestConvertMT5SymbolSessions(t *testing.T) {
	sp := &mt5pb.SymbolParams{
		Symbol: "GBPUSD",
		SymbolInfo: &mt5pb.SymbolInfo{
			Digits:       5,
			Points:       1e-5,
			ContractSize: 100000,
		},
		SymbolGroup: &mt5pb.SymGroup{TradeMode: 3},
	}

	sessions := []*mt5pb.SymbolSessionsEx{
		{
			Symbol: "GBPUSD",
			Quotes: []*mt5pb.SessionsForDay{
				{Sessions: []*mt5pb.Session{{StartTime: 0, EndTime: 86400}}},
			},
			Trades: []*mt5pb.SessionsForDay{
				{Sessions: []*mt5pb.Session{{StartTime: 3600, EndTime: 82800}}},
			},
		},
	}

	sym := ConvertMT5Symbol(sp, "b1", sessions, nil)
	if sym.SessionsQuote == nil || sym.SessionsTrade == nil {
		t.Error("sessions not populated")
	}
}

func TestConvertMT5SymbolCornerCase(t *testing.T) {
	// Empty symbol info + no group → still produces a row
	sp := &mt5pb.SymbolParams{
		Symbol: "UNKNOWN",
	}

	sym := ConvertMT5Symbol(sp, "b1", nil, nil)
	if sym.SymbolRaw != "UNKNOWN" {
		t.Errorf("raw: got %q, want UNKNOWN", sym.SymbolRaw)
	}
	if !sym.Partial {
		t.Error("expected partial for missing all fields")
	}
}
