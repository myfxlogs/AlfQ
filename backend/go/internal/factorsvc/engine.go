// Package factorsvc implements the factor computation service.
// It subscribes to bar streams, evaluates factor DSL expressions,
// and publishes factor values to NATS.
package factorsvc

import (
	"context"
	"fmt"
	"sync"

	"github.com/alfq/backend/go/internal/factor/dsl"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// FactorDef defines a factor loaded from configuration.
type FactorDef struct {
	Name       string
	Expression string
	Symbols    []string
}

// Config holds the factor-svc configuration.
type Config struct {
	Factors []FactorDef
	NatsURL string
}

// Engine manages factor computation.
type Engine struct {
	mu       sync.RWMutex
	factors  map[string]*compiledFactor
	compiler *dsl.Compiler
}

type compiledFactor struct {
	def FactorDef
	op  dsl.Op
}

// NewEngine creates a new factor engine.
func NewEngine(cfg Config) *Engine {
	fields := dsl.FieldIndex{Fields: map[string]int{
		"open": 0, "high": 1, "low": 2, "close": 3, "volume": 4, "bid": 5, "ask": 6,
	}}
	e := &Engine{
		factors:  make(map[string]*compiledFactor),
		compiler: dsl.NewCompiler(fields, nil),
	}
	for _, f := range cfg.Factors {
		e.Register(f)
	}
	return e
}

// Register compiles and registers a factor definition.
func (e *Engine) Register(def FactorDef) error {
	op, err := e.compiler.Compile(def.Expression)
	if err != nil {
		return fmt.Errorf("register factor %q: %w", def.Name, err)
	}
	e.mu.Lock()
	e.factors[def.Name] = &compiledFactor{def: def, op: op}
	e.mu.Unlock()
	return nil
}

// Eval evaluates all registered factors for a given bar, returning name→value.
func (e *Engine) Eval(ctx context.Context, bar *pb.Bar) map[string]float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	results := make(map[string]float64, len(e.factors))
	for name, cf := range e.factors {
		val := bar.GetClose().GetValue()
		v, _ := parseFloat(val)
		results[name] = cf.op.Eval(v)
	}
	return results
}

func parseFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	var f float64
	n, err := fmt.Sscanf(s, "%f", &f)
	return f, n == 1 && err == nil
}
