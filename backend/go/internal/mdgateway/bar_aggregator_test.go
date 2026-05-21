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

func TestParseFloat_Valid(t *testing.T) {
	v, ok := parseFloat("1.2345")
	if !ok {
		t.Fatal("parseFloat should return true for valid input")
	}
	if v != 1.2345 {
		t.Fatalf("expected 1.2345, got %f", v)
	}
}

func TestParseFloat_Empty(t *testing.T) {
	v, ok := parseFloat("")
	if ok {
		t.Fatal("parseFloat should return false for empty input")
	}
	if v != 0 {
		t.Fatalf("expected 0, got %f", v)
	}
}

func TestFastFloat_Valid(t *testing.T) {
	v, err := fastFloat("123.45")
	if err != nil {
		t.Fatalf("fastFloat error: %v", err)
	}
	if v != 123.45 {
		t.Fatalf("expected 123.45, got %f", v)
	}
}

func TestFastFloat_Integer(t *testing.T) {
	v, err := fastFloat("123")
	if err != nil {
		t.Fatalf("fastFloat error: %v", err)
	}
	if v != 123 {
		t.Fatalf("expected 123, got %f", v)
	}
}

func TestFastFloat_Empty(t *testing.T) {
	v, err := fastFloat("")
	if err != nil {
		t.Fatalf("fastFloat error: %v", err)
	}
	if v != 0 {
		t.Fatalf("expected 0, got %f", v)
	}
}

func TestBar_Fields(t *testing.T) {
	bar := Bar{
		TenantID:      "tenant-1",
		Broker:        "broker-1",
		SymbolRaw:     "EURUSD",
		Canonical:     "EURUSD",
		Period:        "1m",
		OpenTsUnixMs:  1234567890,
		CloseTsUnixMs: 1234567950,
		Open:          1.1000,
		High:          1.1050,
		Low:           1.0990,
		Close:         1.1040,
		Volume:        100.0,
		TickCount:     10,
	}
	if bar.TenantID != "tenant-1" {
		t.Fatalf("expected tenant-1, got %s", bar.TenantID)
	}
	if bar.Period != "1m" {
		t.Fatalf("expected 1m, got %s", bar.Period)
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
