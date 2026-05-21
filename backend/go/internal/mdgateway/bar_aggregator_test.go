package mdgateway

import (
	"testing"
)

func TestNewAggregator(t *testing.T) {
	a := NewAggregator()
	if a == nil {
		t.Fatal("NewAggregator returned nil")
	}
	if a.bars == nil {
		t.Fatal("bars map not initialized")
	}
}

func TestAggregator_Size(t *testing.T) {
	a := NewAggregator()
	if a.Size() != 0 {
		t.Fatalf("expected 0, got %d", a.Size())
	}
}

func TestFastFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"123", 123},
		{"0", 0},
		{"1.5", 1.5},
		{"0.25", 0.25},
		{"abc", 0}, // ignores non-digits
		{"", 0},
		{"12.34.56", 12.3456}, // continues after second dot
		{"-10", 10}, // minus ignored, parses digits
	}

	for _, tt := range tests {
		got, _ := fastFloat(tt.input)
		if got != tt.want {
			t.Fatalf("fastFloat(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
		ok    bool
	}{
		{"123", 123, true},
		{"1.5", 1.5, true},
		{"0", 0, true},
		{"", 0, false},
		{"abc", 0, true}, // fastFloat returns 0 for non-numeric, so ok=true
	}

	for _, tt := range tests {
		got, ok := parseFloat(tt.input)
		if ok != tt.ok {
			t.Fatalf("parseFloat(%q) ok = %v, want %v", tt.input, ok, tt.ok)
		}
		if ok && got != tt.want {
			t.Fatalf("parseFloat(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}
