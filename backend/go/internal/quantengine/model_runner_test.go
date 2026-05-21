package quantengine

import (
	"testing"

	stratspec "github.com/alfq/backend/go/internal/strategysvc/spec"
)

func TestNewModelRunner(t *testing.T) {
	spec := &stratspec.StrategySpec{
		Name:      "test",
		ModelURI:  "",
		SignalRule: "close > 0 ? 1 : -1",
	}
	mr, err := NewModelRunner(spec)
	if err != nil {
		t.Fatalf("NewModelRunner error: %v", err)
	}
	if mr == nil {
		t.Fatal("NewModelRunner returned nil")
	}
}

func TestModelRunner_Fields(t *testing.T) {
	spec := &stratspec.StrategySpec{
		Name:      "test",
		ModelURI:  "",
		SignalRule: "close > 0 ? 1 : -1",
	}
	mr, _ := NewModelRunner(spec)
	if mr.spec != spec {
		t.Fatal("expected spec to match")
	}
	if !mr.useDSL {
		t.Fatal("expected useDSL to be true when ModelURI is empty")
	}
}
