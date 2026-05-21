package mdgateway

import (
	"testing"
	"time"
)

func TestDefaultCHWriterConfig(t *testing.T) {
	cfg := DefaultCHWriterConfig()
	if cfg.FlushInterval != time.Second {
		t.Fatalf("expected FlushInterval=1s, got %v", cfg.FlushInterval)
	}
	if cfg.MaxBatchSize != 1000 {
		t.Fatalf("expected MaxBatchSize=1000, got %d", cfg.MaxBatchSize)
	}
}

func TestNewCHWriter(t *testing.T) {
	cfg := CHWriterConfig{
		FlushInterval: 500 * time.Millisecond,
		MaxBatchSize:  500,
	}
	
	w := NewCHWriter(cfg, nil, nil)
	if w == nil {
		t.Fatal("NewCHWriter returned nil")
	}
	if w.cfg.FlushInterval != 500*time.Millisecond {
		t.Fatalf("expected FlushInterval=500ms, got %v", w.cfg.FlushInterval)
	}
	if w.cfg.MaxBatchSize != 500 {
		t.Fatalf("expected MaxBatchSize=500, got %d", w.cfg.MaxBatchSize)
	}
	if w.ticks == nil {
		t.Fatal("ticks channel not initialized")
	}
	if w.done == nil {
		t.Fatal("done channel not initialized")
	}
}

func TestNewCHWriter_DefaultConfig(t *testing.T) {
	w := NewCHWriter(CHWriterConfig{}, nil, nil)
	if w.cfg.FlushInterval != time.Second {
		t.Fatalf("expected default FlushInterval=1s, got %v", w.cfg.FlushInterval)
	}
	if w.cfg.MaxBatchSize != 1000 {
		t.Fatalf("expected default MaxBatchSize=1000, got %d", w.cfg.MaxBatchSize)
	}
}
