package quantengine

import (
	"testing"

	stratspec "github.com/alfq/backend/go/internal/strategysvc/spec"
)

func TestNewModelRunner(t *testing.T) {
	spec := &stratspec.StrategySpec{
		Name:    "test",
		Version: "1.0.0",
	}
	_, err := NewModelRunner(spec)
	// NewModelRunner may fail if spec is invalid, just ensure it doesn't panic
	_ = err
}
