package sizing

import (
	"testing"
)

func TestNewCalculator(t *testing.T) {
	c := NewCalculator()
	if c == nil {
		t.Fatal("NewCalculator returned nil")
	}
}

func TestCompute_NilSizing(t *testing.T) {
	c := NewCalculator()
	result := c.Compute(nil, 10000, 0.1)
	if result != 0.1 {
		t.Fatalf("expected 0.1, got %f", result)
	}
}

func TestCompute_FixedLots(t *testing.T) {
	c := NewCalculator()
	sizing := map[string]any{"type": "fixed_lots", "lots": 0.5}
	result := c.Compute(sizing, 10000, 0.1)
	if result != 0.5 {
		t.Fatalf("expected 0.5, got %f", result)
	}
}

func TestCompute_FixedLots_InvalidType(t *testing.T) {
	c := NewCalculator()
	sizing := map[string]any{"type": "fixed_lots", "lots": "0.5"}
	result := c.Compute(sizing, 10000, 0.1)
	if result != 0.1 {
		t.Fatalf("expected default 0.1, got %f", result)
	}
}

func TestCompute_PctEquity(t *testing.T) {
	c := NewCalculator()
	sizing := map[string]any{"type": "pct_equity", "pct": 10.0}
	result := c.Compute(sizing, 100000, 0.1)
	expected := 100000 * 0.1 / 100000.0 // 1.0
	if result != expected {
		t.Fatalf("expected %f, got %f", expected, result)
	}
}

func TestCompute_PctEquity_ZeroEquity(t *testing.T) {
	c := NewCalculator()
	sizing := map[string]any{"type": "pct_equity", "pct": 10.0}
	result := c.Compute(sizing, 0, 0.1)
	if result != 0.1 {
		t.Fatalf("expected default 0.1, got %f", result)
	}
}

func TestCompute_PctEquity_MinimumLots(t *testing.T) {
	c := NewCalculator()
	sizing := map[string]any{"type": "pct_equity", "pct": 0.1}
	result := c.Compute(sizing, 1000, 0.1)
	// 1000 * 0.1 / 100000 = 0.001, should be clamped to 0.01
	if result != 0.01 {
		t.Fatalf("expected 0.01, got %f", result)
	}
}

func TestCompute_UnknownType(t *testing.T) {
	c := NewCalculator()
	sizing := map[string]any{"type": "unknown"}
	result := c.Compute(sizing, 10000, 0.1)
	if result != 0.1 {
		t.Fatalf("expected default 0.1, got %f", result)
	}
}
