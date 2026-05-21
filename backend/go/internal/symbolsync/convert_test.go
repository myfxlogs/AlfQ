package symbolsync

import (
	"testing"

	mt4pb "github.com/alfq/backend/go/gen/mt4"
	mt5pb "github.com/alfq/backend/go/gen/mt5"
)

func TestConvertMT5Symbol(t *testing.T) {
	sp := &mt5pb.SymbolParams{
		Symbol: "EURUSD",
	}
	sym := ConvertMT5Symbol(sp, "broker-1", nil, nil)
	if sym.BrokerID != "broker-1" {
		t.Fatalf("expected broker-1, got %s", sym.BrokerID)
	}
	if sym.SymbolRaw != "EURUSD" {
		t.Fatalf("expected EURUSD, got %s", sym.SymbolRaw)
	}
	if sym.Canonical != "EURUSD" {
		t.Fatalf("expected EURUSD, got %s", sym.Canonical)
	}
}

func TestConvertMT5Symbol_Partial(t *testing.T) {
	sp := &mt5pb.SymbolParams{
		Symbol: "EURUSD",
	}
	sym := ConvertMT5Symbol(sp, "broker-1", nil, nil)
	if !sym.Partial {
		t.Fatal("expected Partial to be true when digits/point/contractSize are 0")
	}
}

func TestConvertMT4Symbol_TradeModeFull(t *testing.T) {
	sp := &mt4pb.SymbolParams{
		SymbolName: "EURUSDm",
		Symbol: &mt4pb.SymbolInfo{
			Digits:       5,
			Point:        0.00001,
			ContractSize: 100000,
		},
	}
	result := ConvertMT4Symbol(sp, "broker-1", nil, nil)

	if result.TradeMode != 4 {
		t.Errorf("expected TradeMode=4 for valid symbol, got %d", result.TradeMode)
	}
	if result.Partial {
		t.Errorf("expected Partial=false for valid symbol, got true")
	}
	if result.SymbolRaw != "EURUSDm" {
		t.Errorf("expected SymbolRaw=EURUSDm, got %s", result.SymbolRaw)
	}
}

func TestConvertMT4Symbol_Partial(t *testing.T) {
	sp := &mt4pb.SymbolParams{
		SymbolName: "INVALID",
		Symbol: &mt4pb.SymbolInfo{
			Digits:       0,
			Point:        0,
			ContractSize: 0,
		},
	}
	result := ConvertMT4Symbol(sp, "broker-1", nil, nil)

	if !result.Partial {
		t.Errorf("expected Partial=true for invalid symbol, got false")
	}
	if result.TradeMode != 0 {
		t.Errorf("expected TradeMode=0 for partial symbol, got %d", result.TradeMode)
	}
}

func TestConvertMT4Symbol_TradeModeZeroWhenMissingFields(t *testing.T) {
	// ContractSize missing — should be partial, TradeMode stays 0
	sp := &mt4pb.SymbolParams{
		SymbolName: "EURUSDm",
		Symbol: &mt4pb.SymbolInfo{
			Digits: 5,
			Point:  0.00001,
		},
	}
	result := ConvertMT4Symbol(sp, "broker-1", nil, nil)

	if !result.Partial {
		t.Errorf("expected Partial=true when ContractSize=0, got false")
	}
	if result.TradeMode != 0 {
		t.Errorf("expected TradeMode=0 for partial symbol, got %d", result.TradeMode)
	}
}
