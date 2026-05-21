package factorsvc

import (
	"testing"
	"time"
)

func TestDefaultFactorCHWriterConfig(t *testing.T) {
	cfg := DefaultFactorCHWriterConfig()
	if cfg.FlushInterval != 5*time.Second {
		t.Fatalf("expected 5s, got %v", cfg.FlushInterval)
	}
	if cfg.MaxBatchSize != 500 {
		t.Fatalf("expected 500, got %d", cfg.MaxBatchSize)
	}
}

func TestFactorCHWriterConfig_Fields(t *testing.T) {
	cfg := FactorCHWriterConfig{
		FlushInterval: 10 * time.Second,
		MaxBatchSize:  1000,
	}
	if cfg.FlushInterval != 10*time.Second {
		t.Fatalf("expected 10s, got %v", cfg.FlushInterval)
	}
	if cfg.MaxBatchSize != 1000 {
		t.Fatalf("expected 1000, got %d", cfg.MaxBatchSize)
	}
}
