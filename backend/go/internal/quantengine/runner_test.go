package quantengine

import (
	"testing"

	stratspec "github.com/alfq/backend/go/internal/strategysvc/spec"
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

func TestStrategyRuntime(t *testing.T) {
	spec := &stratspec.StrategySpec{Name: "test"}
	rt := &StrategyRuntime{
		Spec:   spec,
		Runner: nil,
	}
	if rt.Spec.Name != "test" {
		t.Fatalf("expected test, got %s", rt.Spec.Name)
	}
}
