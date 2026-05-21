// Package quantengine — ONNX runtime integration for strategy inference.
package quantengine

import (
	"context"
	"fmt"
	"math"

	"github.com/alfq/backend/go/internal/factor/dsl"
	stratspec "github.com/alfq/backend/go/internal/strategysvc/spec"
)

// ModelRunner executes ONNX model inference.
// Currently a DSL-based fallback per ADR 0013 (ONNX runtime strategy).
// Full integration via assistant-svc Python ORT bridge will be triggered when:
//  1. trainer produces .onnx files written to ai_artifacts
//  2. at least one researcher runs ONNX on paper for ≥1 week
//  3. model governance (drift/shadow/lifecycle) is in place
// See docs/adr/0013-onnx-runtime-strategy.md for details.
type ModelRunner struct {
	spec     *stratspec.StrategySpec
	compiler *dsl.Compiler
	useDSL   bool // fallback to DSL signal rule when ONNX model is unavailable
}

// NewModelRunner creates a model runner for a strategy spec.
// If model_uri is empty, it falls back to the DSL signal_rule.
func NewModelRunner(spec *stratspec.StrategySpec) (*ModelRunner, error) {
	fields := dsl.FieldIndex{Fields: map[string]int{
		"close": 0, "open": 1, "high": 2, "low": 3, "volume": 4,
	}}

	mr := &ModelRunner{
		spec:     spec,
		compiler: dsl.NewCompiler(fields, nil),
	}

	if spec.ModelURI == "" {
		mr.useDSL = true
	} else {
		// ONNX runtime placeholder: in production this would load
		// the ONNX model from MinIO and create an onnxruntime session.
		// For now fall back to DSL for parity with research SDK.
		mr.useDSL = true
	}

	return mr, nil
}

// Predict runs inference with the strategy model and returns a signal.
// Input: factor values as float64 map. Output: signal float64 (>0 long, <0 short).
func (mr *ModelRunner) Predict(ctx context.Context, factorValues map[string]float64) (float64, error) {
	if mr.useDSL {
		return mr.predictDSL(ctx, factorValues)
	}

	// ONNX inference path (placeholder — requires onnxruntime-go)
	return 0, fmt.Errorf("onnx runtime not available; use DSL signal_rule instead")
}

func (mr *ModelRunner) predictDSL(ctx context.Context, factorValues map[string]float64) (float64, error) {
	if mr.spec.SignalRule == "" {
		return 0, nil
	}

	factors := make(map[string]dsl.Op, len(factorValues))
	for name, val := range factorValues {
		factors[name] = &constOp{val: val}
	}

	op, err := mr.compiler.CompileWithFactors(mr.spec.SignalRule, factors)
	if err != nil {
		return 0, fmt.Errorf("model predict dsl: %w", err)
	}

	return op.Eval(0), nil
}

type constOp struct{ val float64 }

func (c *constOp) Eval(v float64) float64 { return c.val }
func (c *constOp) Reset()                 {}
func (c *constOp) Warmup() int            { return 0 }

// Direction returns "long", "short", or "flat" for a signal value.
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
