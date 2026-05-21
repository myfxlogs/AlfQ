// Package strategysvc implements the strategy execution service.
package strategysvc

import (
	"context"
	"sync"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Signal represents a trading signal produced by a strategy.
type Signal struct {
	StrategyID string
	Symbol     string
	Side       pb.OrderSide
	TargetQty  float64
	Confidence float64
	Reason     string
}

// SignalHandler receives signals produced by strategy evaluation.
// Implementations bridge to OMS, logging, or paper-trading backends.
type SignalHandler func(signal *Signal)

// Runner executes a single strategy deployment.
type Runner struct {
	mu       sync.Mutex
	strategy Strategy
	position map[string]float64 // symbol → current qty
	onSignal SignalHandler      // optional: bridges signal to OMS
}

// Strategy interface (inspired by bbgo).
type Strategy interface {
	ID() string
	OnFactor(ctx context.Context, factor string, value float64) (*Signal, error)
	OnBar(ctx context.Context, bar *pb.Bar) (*Signal, error)
}

// NewRunner creates a runner for a strategy instance.
func NewRunner(s Strategy) *Runner {
	return &Runner{strategy: s, position: make(map[string]float64)}
}

// SetSignalHandler sets the callback invoked when a non-flat signal is produced.
func (r *Runner) SetSignalHandler(h SignalHandler) {
	r.mu.Lock()
	r.onSignal = h
	r.mu.Unlock()
}

// Evaluate produces a signal from factor values.
func (r *Runner) Evaluate(ctx context.Context, factor string, value float64) (*Signal, error) {
	sig, err := r.strategy.OnFactor(ctx, factor, value)
	if err != nil {
		return nil, err
	}
	if sig != nil && sig.TargetQty != 0 && r.onSignal != nil {
		r.onSignal(sig)
	}
	return sig, nil
}

// GetPosition returns the current position for a symbol.
func (r *Runner) GetPosition(symbol string) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.position[symbol]
}

// UpdatePosition updates the tracked position.
func (r *Runner) UpdatePosition(symbol string, qty float64) {
	r.mu.Lock()
	r.position[symbol] = qty
	r.mu.Unlock()
}
