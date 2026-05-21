package signal

import (
	"math"
	"testing"

	stratspec "github.com/alfq/backend/go/internal/strategysvc/spec"
)

func TestDirection(t *testing.T) {
	tests := []struct {
		signal  float64
		want    string
	}{
		{1.0, "long"},
		{0.5, "long"},
		{-1.0, "short"},
		{-0.5, "short"},
		{0.0, "flat"},
		{math.NaN(), "flat"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := Direction(tt.signal); got != tt.want {
				t.Fatalf("Direction(%f) = %s, want %s", tt.signal, got, tt.want)
			}
		})
	}
}

func TestNewGenerator(t *testing.T) {
	spec := &stratspec.StrategySpec{
		Name:       "test",
		SignalRule: "sma20 > sma60 ? 1 : -1",
		Factors:    map[string]string{},
	}
	
	g, err := NewGenerator(spec)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	if g == nil {
		t.Fatal("NewGenerator() returned nil")
	}
}

func TestGeneratorEval_EmptyRule(t *testing.T) {
	spec := &stratspec.StrategySpec{
		Name:       "test",
		SignalRule: "",
		Factors:    map[string]string{},
	}
	
	g, err := NewGenerator(spec)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	
	signal, err := g.Eval(nil, nil)
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}
	if signal != 0 {
		t.Fatalf("Eval() signal = %f, want 0", signal)
	}
}

func TestGeneratorEval_SimpleRule(t *testing.T) {
	spec := &stratspec.StrategySpec{
		Name:       "test",
		SignalRule: "sma20 > sma60 ? 1 : -1",
		Factors:    map[string]string{},
	}
	
	g, err := NewGenerator(spec)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	
	// sma20 > sma60 should evaluate to true when sma20 is greater
	factorValues := map[string]float64{"sma20": 50.0, "sma60": 40.0}
	signal, err := g.Eval(nil, factorValues)
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}
	if signal <= 0 {
		t.Fatalf("Eval() signal = %f, want > 0", signal)
	}
}

func TestConstOp(t *testing.T) {
	op := &constOp{val: 42.0}
	if op.Eval(0) != 42.0 {
		t.Fatalf("constOp.Eval() = %f, want 42.0", op.Eval(0))
	}
	op.Reset() // Should not panic
	if op.Warmup() != 0 {
		t.Fatalf("constOp.Warmup() = %d, want 0", op.Warmup())
	}
}

func TestGenerator_Fields(t *testing.T) {
	spec := &stratspec.StrategySpec{
		Name:       "test",
		SignalRule: "sma20 > sma60 ? 1 : -1",
		Factors:    map[string]string{},
	}
	
	g, err := NewGenerator(spec)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	if g.spec != spec {
		t.Fatal("expected spec to match")
	}
	if g.compiler == nil {
		t.Fatal("expected compiler to be set")
	}
}
