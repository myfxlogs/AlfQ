package quantengine

import (
	"testing"
)

func TestDefaultDemoSpec(t *testing.T) {
	spec := defaultDemoSpec()
	if spec == nil {
		t.Fatal("defaultDemoSpec returned nil")
	}
	if spec.Name != "demo_sma_cross" {
		t.Fatalf("expected Name=demo_sma_cross, got %s", spec.Name)
	}
	if spec.Version != "1.0.0" {
		t.Fatalf("expected Version=1.0.0, got %s", spec.Version)
	}
	if len(spec.CanonicalSymbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(spec.CanonicalSymbols))
	}
	if spec.CanonicalSymbols[0] != "EURUSD" {
		t.Fatalf("expected EURUSD, got %s", spec.CanonicalSymbols[0])
	}
	if len(spec.Factors) != 2 {
		t.Fatalf("expected 2 factors, got %d", len(spec.Factors))
	}
	if spec.SignalRule != "sma20 > sma60 ? 1 : -1" {
		t.Fatalf("unexpected SignalRule: %s", spec.SignalRule)
	}
}

func TestStrategyRuntime_Fields(t *testing.T) {
	rt := &StrategyRuntime{
		Spec:   nil,
		Runner: nil,
	}
	if rt.Spec != nil {
		t.Fatal("expected nil Spec")
	}
	if rt.Runner != nil {
		t.Fatal("expected nil Runner")
	}
}

func TestSignalHandler(t *testing.T) {
	// Just ensure the type exists
	var h SignalHandler = func(symbol, side string, qty float64, reason string) {}
	if h == nil {
		t.Fatal("SignalHandler should not be nil")
	}
	h("EURUSD", "buy", 0.1, "test")
}
