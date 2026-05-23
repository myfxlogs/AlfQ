// Package risksvc implements the risk management service.
// All orders must pass through risk checks before submission.
package risksvc

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Rule is a named risk check.
type Rule interface {
	Name() string
	Check(ctx context.Context, req *pb.OrderRequest, state *AccountState) *pb.RiskCheckResult
}

// AccountState holds real-time risk metrics for an account.
type AccountState struct {
	Balance        float64
	Equity         float64
	Margin         float64
	FreeMargin     float64
	DailyPnL       float64
	MaxDrawdown    float64
	Positions      map[string]*pb.Position // symbol → position
	TotalPositions int32
	OpenOrders     int32
}

// Engine evaluates risk rules in sequence.
type Engine struct {
	mu    sync.RWMutex
	rules []Rule
	state map[string]*AccountState // account_id → state
}

// NewEngine creates a new risk engine with default rules.
func NewEngine() *Engine {
	e := &Engine{
		state: make(map[string]*AccountState),
	}
	// Register default rules per docs/08 §5.2 (M3 base) + M4 additions
	e.Register(&MaxLot{maxLot: 100.0})
	e.Register(&MaxPosition{maxPerSymbol: 10.0})
	e.Register(&DailyLoss{maxDailyLoss: 5000.0})
	e.Register(&Drawdown{maxDrawdown: 0.15})
	// CanonicalAuth replaces legacy Whitelist (M4)
	// Wire pool via WithCanonicalAuth() after engine creation if PG is available.
	// M4 rules
	e.Register(NewSession("UTC"))
	e.Register(NewMargin(1.5))
	e.Register(NewSlippage(5.0))
	e.Register(NewHeartbeat(5 * time.Minute))
	e.Register(NewRejectRate(0.3, 5*time.Minute))
	return e
}

// WithCanonicalAuth registers CanonicalAuth with PG pool and returns the engine.
func (e *Engine) WithCanonicalAuth(pool *pgxpool.Pool) *Engine {
	e.Register(NewCanonicalAuth(pool))
	return e
}

// NewTestEngine creates a risk engine without time-dependent rules
// (session, heartbeat, reject_rate) for deterministic unit tests.
func NewTestEngine() *Engine {
	e := &Engine{
		state: make(map[string]*AccountState),
	}
	e.Register(&MaxLot{maxLot: 100.0})
	e.Register(&MaxPosition{maxPerSymbol: 10.0})
	e.Register(&DailyLoss{maxDailyLoss: 5000.0})
	e.Register(&Drawdown{maxDrawdown: 0.15})
	e.Register(NewMargin(1.5))
	e.Register(NewSlippage(5.0))
	return e
}

// Rules returns a copy of the registered rules (for inspection).
func (e *Engine) Rules() []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Rule, len(e.rules))
	copy(out, e.rules)
	return out
}

// Register adds a rule to the engine.
func (e *Engine) Register(r Rule) {
	e.mu.Lock()
	e.rules = append(e.rules, r)
	e.mu.Unlock()
}

// CanonicalAuth returns the canonical auth rule or nil.
func (e *Engine) CanonicalAuth() *CanonicalAuth {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, r := range e.rules {
		if ca, ok := r.(*CanonicalAuth); ok {
			return ca
		}
	}
	return nil
}

// Check runs all rules against an order request. Returns deny on first failure.
func (e *Engine) Check(ctx context.Context, req *pb.OrderRequest) *pb.RiskCheckResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state := e.state[req.AccountId]
	if state == nil {
		state = &AccountState{}
	}

	for _, r := range e.rules {
		result := r.Check(ctx, req, state)
		if !result.Approved {
			return result
		}
	}
	return &pb.RiskCheckResult{Approved: true}
}

// UpdateState updates the account state snapshot.
func (e *Engine) UpdateState(accountID string, state *AccountState) {
	e.mu.Lock()
	e.state[accountID] = state
	e.mu.Unlock()
}

// ── Rule implementations ──

// MaxLot rejects orders exceeding the maximum lot size.
type MaxLot struct{ maxLot float64 }

func (r *MaxLot) Name() string { return "max_lot" }
func (r *MaxLot) Check(_ context.Context, req *pb.OrderRequest, _ *AccountState) *pb.RiskCheckResult {
	if req.Qty > r.maxLot {
		return &pb.RiskCheckResult{Approved: false, Reason: fmt.Sprintf("qty %.2f exceeds max lot %.2f", req.Qty, r.maxLot), RuleId: r.Name()}
	}
	return &pb.RiskCheckResult{Approved: true}
}

// MaxPosition rejects orders that would exceed the per-symbol position limit.
type MaxPosition struct{ maxPerSymbol float64 }

func (r *MaxPosition) Name() string { return "max_position" }
func (r *MaxPosition) Check(_ context.Context, req *pb.OrderRequest, state *AccountState) *pb.RiskCheckResult {
	pos := state.Positions[req.Symbol]
	if pos != nil {
		newQty := pos.Qty + req.Qty
		if req.Side == pb.OrderSide_ORDER_SIDE_SELL {
			newQty = pos.Qty - req.Qty
		}
		if abs(newQty) > r.maxPerSymbol {
			return &pb.RiskCheckResult{Approved: false, Reason: fmt.Sprintf("position %.2f would exceed limit %.2f", newQty, r.maxPerSymbol), RuleId: r.Name()}
		}
	}
	return &pb.RiskCheckResult{Approved: true}
}

// DailyLoss rejects if daily PnL exceeds the loss limit.
type DailyLoss struct{ maxDailyLoss float64 }

func (r *DailyLoss) Name() string { return "daily_loss" }
func (r *DailyLoss) Check(_ context.Context, req *pb.OrderRequest, state *AccountState) *pb.RiskCheckResult {
	if state.DailyPnL < -r.maxDailyLoss {
		return &pb.RiskCheckResult{Approved: false, Reason: fmt.Sprintf("daily loss %.2f exceeds limit %.2f", -state.DailyPnL, r.maxDailyLoss), RuleId: r.Name()}
	}
	return &pb.RiskCheckResult{Approved: true}
}

// Drawdown rejects if max drawdown threshold is exceeded.
type Drawdown struct{ maxDrawdown float64 }

func (r *Drawdown) Name() string { return "drawdown" }
func (r *Drawdown) Check(_ context.Context, req *pb.OrderRequest, state *AccountState) *pb.RiskCheckResult {
	if state.MaxDrawdown > r.maxDrawdown {
		return &pb.RiskCheckResult{Approved: false, Reason: fmt.Sprintf("drawdown %.2f exceeds limit %.2f", state.MaxDrawdown, r.maxDrawdown), RuleId: r.Name()}
	}
	return &pb.RiskCheckResult{Approved: true}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
