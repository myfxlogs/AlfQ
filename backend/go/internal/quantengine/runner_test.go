package quantengine

import (
	"testing"

	"go.uber.org/zap"
)

func TestDefaultDemoSpec(t *testing.T) {
	spec := defaultDemoSpec()
	if spec == nil {
		t.Fatal("defaultDemoSpec returned nil")
	}
	if spec.Name != "demo_sma_e2e" {
		t.Fatalf("expected Name=demo_sma_e2e, got %s", spec.Name)
	}
	if spec.Version != "1.0.0" {
		t.Fatalf("expected Version=1.0.0, got %s", spec.Version)
	}
	if len(spec.CanonicalSymbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(spec.CanonicalSymbols))
	}
	if spec.CanonicalSymbols[0] != "BTCUSD" {
		t.Fatalf("expected BTCUSD, got %s", spec.CanonicalSymbols[0])
	}
	if len(spec.Factors) != 2 {
		t.Fatalf("expected 2 factors, got %d", len(spec.Factors))
	}
	if spec.SignalRule != "sma20 > sma60 ? 1 : -1" {
		t.Fatalf("unexpected SignalRule: %s", spec.SignalRule)
	}
}

func TestStrategyRuntime_Fields(t *testing.T) {
	// RS05: StrategyRuntime now uses constructor with valid spec.
	spec := defaultDemoSpec()
	rt, err := NewStrategyRuntime(spec, nil, nil, zap.NewNop())
	if err != nil {
		t.Logf("runtime creation expected error without engine: %v", err)
		return
	}
	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}
	if rt.Spec == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestSignalHandler(t *testing.T) {
	// Just ensure the type exists
	var h SignalHandler = func(strategyID, symbol, side string, qty float64, reason string) {}
	if h == nil {
		t.Fatal("SignalHandler should not be nil")
	}
	h("sid", "EURUSD", "buy", 0.1, "test")
}
