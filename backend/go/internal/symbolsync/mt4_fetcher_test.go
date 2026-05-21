// Package symbolsync — MT4 convert tests with proto-level mock fixtures.
package symbolsync

import (
	"encoding/json"
	"testing"

	mt4pb "github.com/alfq/backend/go/gen/mt4"
)

func TestConvertMT4SymbolEURUSD(t *testing.T) {
	sp := &mt4pb.SymbolParams{
		SymbolName: "EURUSD",
		Symbol: &mt4pb.SymbolInfo{
			Digits:       5,
			Point:        1e-5,
			ContractSize: 100000,
			SwapLong:     -3.5,
			SwapShort:    1.2,
		},
		GroupParams: &mt4pb.ConGroupSec{
			MinLot:   0.01,
			MaxLot:   100,
			LotStep:  0.01,
		},
	}

	tz := &mt4pb.ServerTimezoneReply{
		Result: 7200, // +2 hours in seconds
	}

	sym := ConvertMT4Symbol(sp, "b1", nil, tz)

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
	if sym.ServerTimezone != "+2" {
		t.Errorf("tz: got %q, want +2", sym.ServerTimezone)
	}
}

func TestConvertMT4SymbolWithSuffix(t *testing.T) {
	sp := &mt4pb.SymbolParams{
		SymbolName: "GBPJPYm",
		Symbol: &mt4pb.SymbolInfo{
			Digits:       3,
			Point:        0.001,
			ContractSize: 100000,
		},
		GroupParams: &mt4pb.ConGroupSec{
			MinLot:  0.01,
			MaxLot:  50,
			LotStep: 0.01,
		},
	}

	sym := ConvertMT4Symbol(sp, "b1", nil, nil)
	if sym.Canonical != "GBPJPY" {
		t.Errorf("canonical for GBPJPYm: got %q, want GBPJPY", sym.Canonical)
	}
	if sym.MinLot != 0.01 {
		t.Errorf("min_lot: got %f", sym.MinLot)
	}
}

func TestConvertMT4SymbolPartial(t *testing.T) {
	// Missing all core fields
	sp := &mt4pb.SymbolParams{
		SymbolName: "XAUUSD",
	}

	sym := ConvertMT4Symbol(sp, "b1", nil, nil)
	if !sym.Partial {
		t.Error("expected partial for no fields")
	}
}

func TestConvertMT4SymbolNoGroup(t *testing.T) {
	// Only SymbolInfo, no GroupParams
	sp := &mt4pb.SymbolParams{
		SymbolName: "US30",
		Symbol: &mt4pb.SymbolInfo{
			Digits:       2,
			Point:        0.01,
			ContractSize: 1,
		},
	}

	sym := ConvertMT4Symbol(sp, "b1", nil, nil)
	if sym.Partial {
		t.Error("should not be partial: core fields present")
	}
	if sym.MinLot != 0 {
		t.Errorf("min_lot should be 0 without group params")
	}
}

func TestConvertMT4Sessions(t *testing.T) {
	sessions := []*mt4pb.ConSessions{
		{
			Quote: []*mt4pb.ConSession{
				{OpenHour: 0, OpenMin: 0, CloseHour: 23, CloseMin: 59},
			},
			Trade: []*mt4pb.ConSession{
				{OpenHour: 0, OpenMin: 5, CloseHour: 23, CloseMin: 55},
			},
		},
		{
			Quote: []*mt4pb.ConSession{
				{OpenHour: 0, OpenMin: 0, CloseHour: 23, CloseMin: 59},
			},
			Trade: []*mt4pb.ConSession{
				{OpenHour: 0, OpenMin: 5, CloseHour: 23, CloseMin: 55},
			},
		},
	}

	sp := &mt4pb.SymbolParams{
		SymbolName: "EURUSD",
		Symbol: &mt4pb.SymbolInfo{
			Digits:       5,
			Point:        1e-5,
			ContractSize: 100000,
		},
	}

	sym := ConvertMT4Symbol(sp, "b1", sessions, nil)

	// Verify quote sessions
	var quoteDays []struct {
		Sessions []struct {
			Start int32 `json:"start"`
			End   int32 `json:"end"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(sym.SessionsQuote, &quoteDays); err != nil {
		t.Fatalf("unmarshal quote sessions: %v", err)
	}
	if len(quoteDays) != 2 {
		t.Errorf("quote days: got %d, want 2", len(quoteDays))
	}
	if len(quoteDays[0].Sessions) != 1 {
		t.Errorf("quote day 0 sessions: got %d, want 1", len(quoteDays[0].Sessions))
	}
	if quoteDays[0].Sessions[0].Start != 0 {
		t.Errorf("quote day 0 start: got %d, want 0", quoteDays[0].Sessions[0].Start)
	}
	if quoteDays[0].Sessions[0].End != 23*60+59 {
		t.Errorf("quote day 0 end: got %d, want %d", quoteDays[0].Sessions[0].End, 23*60+59)
	}

	// Verify trade sessions
	var tradeDays []struct {
		Sessions []struct {
			Start int32 `json:"start"`
			End   int32 `json:"end"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(sym.SessionsTrade, &tradeDays); err != nil {
		t.Fatalf("unmarshal trade sessions: %v", err)
	}
	if len(tradeDays[0].Sessions) != 1 {
		t.Errorf("trade day 0 sessions: got %d", len(tradeDays[0].Sessions))
	}
	if tradeDays[0].Sessions[0].Start != 5 {
		t.Errorf("trade day 0 start: got %d, want 5 (0:05)", tradeDays[0].Sessions[0].Start)
	}
}

func TestConvertMT4SessionsNil(t *testing.T) {
	sp := &mt4pb.SymbolParams{
		SymbolName: "EURUSD",
		Symbol: &mt4pb.SymbolInfo{
			Digits:       5,
			Point:        1e-5,
			ContractSize: 100000,
		},
	}

	sym := ConvertMT4Symbol(sp, "b1", nil, nil)
	if sym.SessionsQuote != nil {
		t.Error("expected nil sessions_quote for nil input")
	}
	if sym.SessionsTrade != nil {
		t.Error("expected nil sessions_trade for nil input")
	}
}
