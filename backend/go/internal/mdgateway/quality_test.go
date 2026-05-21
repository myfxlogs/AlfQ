package mdgateway

import (
	"testing"
)

func TestDefaultQualityConfig(t *testing.T) {
	cfg := DefaultQualityConfig()
	if cfg.GapMaxSeconds != 5 {
		t.Fatalf("expected GapMaxSeconds=5, got %f", cfg.GapMaxSeconds)
	}
	if cfg.OutlierSigma != 5 {
		t.Fatalf("expected OutlierSigma=5, got %f", cfg.OutlierSigma)
	}
	if cfg.SkewMaxSeconds != 30 {
		t.Fatalf("expected SkewMaxSeconds=30, got %f", cfg.SkewMaxSeconds)
	}
	if cfg.HistorySize != 100 {
		t.Fatalf("expected HistorySize=100, got %d", cfg.HistorySize)
	}
}

func TestMedianSigma(t *testing.T) {
	tests := []struct {
		name    string
		vals    []float64
		wantMed float64
	}{
		{"empty", []float64{}, 0},
		{"single", []float64{10}, 10},
		{"two", []float64{10, 20}, 15},
		{"odd", []float64{1, 2, 3, 4, 5}, 3},
		{"even", []float64{1, 2, 3, 4}, 2.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			med, sigma := medianSigma(tt.vals)
			if med != tt.wantMed {
				t.Fatalf("median = %f, want %f", med, tt.wantMed)
			}
			// Just check sigma is non-negative for valid inputs
			if len(tt.vals) > 0 && sigma < 0 {
				t.Fatalf("sigma should be non-negative, got %f", sigma)
			}
		})
	}
}
