// Package symbolsync — type conversion and session formatting tests.
package symbolsync

import (
	"encoding/json"
	"testing"

	mt5pb "github.com/alfq/backend/go/gen/mt5"
)

func TestBrokerSymbolJSONRoundtrip(t *testing.T) {
	bs := BrokerSymbol{
		BrokerID:        "b1",
		SymbolRaw:       "EURUSD.m",
		Canonical:       "EURUSD",
		Digits:          5,
		Point:           1e-5,
		TickSize:        1e-5,
		TickValue:       1.0,
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

	data, err := json.Marshal(bs)
	if err != nil {
		t.Fatal(err)
	}

	var decoded BrokerSymbol
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Canonical != "EURUSD" {
		t.Errorf("canonical: got %q, want EURUSD", decoded.Canonical)
	}
	if decoded.Digits != 5 {
		t.Errorf("digits: got %d, want 5", decoded.Digits)
	}
	if decoded.ContractSize != 100000 {
		t.Errorf("contract_size: got %f, want 100000", decoded.ContractSize)
	}
}

func TestSessionsForDayToJSON(t *testing.T) {
	// sessionsForDayToJSON iterates the SessionsForDay list and extracts GetSessions().
	// Test with a properly structured SessionsForDay.
	day := &mt5pb.SessionsForDay{
		Sessions: []*mt5pb.Session{
			{StartTime: 0, EndTime: 86400},
		},
	}
	jsonBytes := sessionsForDayToJSON([]*mt5pb.SessionsForDay{day})
	if jsonBytes == nil {
		t.Fatal("sessionsForDayToJSON returned nil")
	}

	var decoded []struct {
		Sessions []struct {
			Start int32 `json:"start"`
			End   int32 `json:"end"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Sessions[0].Start != 0 || decoded[0].Sessions[0].End != 86400 {
		t.Errorf("unexpected JSON: %s", jsonBytes)
	}
}

func TestNilSessionsForDayToJSON(t *testing.T) {
	if b := sessionsForDayToJSON(nil); b != nil {
		t.Errorf("expected nil for nil input, got %s", b)
	}
}

func TestSymbolNamesMT5(t *testing.T) {
	sps := []*mt5pb.SymbolParams{
		{Symbol: "EURUSD"},
		{Symbol: "GBPUSD"},
		{Symbol: ""}, // empty, should be skipped
		{Symbol: "XAUUSD"},
	}

	names := symbolNamesMT5(sps)
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "EURUSD" || names[1] != "GBPUSD" || names[2] != "XAUUSD" {
		t.Errorf("unexpected names: %v", names)
	}
}
