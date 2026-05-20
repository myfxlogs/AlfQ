// Package signal generates trading signals from factor values using DSL rules.
package signal

import (
	"context"
	"fmt"
	"math"

	"github.com/alfq/backend/go/internal/factor/dsl"
	stratspec "github.com/alfq/backend/go/internal/strategysvc/spec"
)

// Generator produces trading signals from factor values.
type Generator struct {
	spec    *stratspec.StrategySpec
	compiler *dsl.Compiler
}

// NewGenerator creates a signal generator for a given strategy spec.
func NewGenerator(spec *stratspec.StrategySpec) (*Generator, error) {
	fields := dsl.FieldIndex{Fields: map[string]int{
		"close": 0, "open": 1, "high": 2, "low": 3, "volume": 4,
	}}
	return &Generator{
		spec:    spec,
		compiler: dsl.NewCompiler(fields, nil),
	}, nil
}

// Eval evaluates the signal rule against current factor values.
// Returns >0 for long, <0 for short, 0 for flat.
func (g *Generator) Eval(ctx context.Context, factorValues map[string]float64) (float64, error) {
	if g.spec.SignalRule == "" {
		return 0, nil
	}

	// Build a factors map where each factor is a constant Op with the current value
	factors := make(map[string]dsl.Op, len(factorValues))
	for name, val := range factorValues {
		factors[name] = &constOp{val: val}
	}

	op, err := g.compiler.CompileWithFactors(g.spec.SignalRule, factors)
	if err != nil {
		return 0, fmt.Errorf("signal eval: %w", err)
	}

	return op.Eval(0), nil
}

// constOp is a dsl.Op that returns a constant value.
type constOp struct{ val float64 }

func (c *constOp) Eval(v float64) float64 { return c.val }
func (c *constOp) Reset()                 {}
func (c *constOp) Warmup() int            { return 0 }

// Direction returns "long", "short", or "flat" from a signal value.
func Direction(signal float64) string {
	if math.IsNaN(signal) {
		return "flat"
	}
	if signal > 0 {
		return "long"
	}
	if signal < 0 {
		return "short"
	}
	return "flat"
}
