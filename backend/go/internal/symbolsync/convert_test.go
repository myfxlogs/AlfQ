package symbolsync

import (
	"testing"

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
