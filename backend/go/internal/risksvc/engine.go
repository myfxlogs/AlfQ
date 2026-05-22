// Package risksvc implements the risk management service.
// All orders must pass through risk checks before submission.
package risksvc

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
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
	e.Register(&Whitelist{})
	// M4 rules
	e.Register(NewSession("UTC"))
	e.Register(NewMargin(1.5))
	e.Register(NewSlippage(5.0))
	e.Register(NewHeartbeat(5 * time.Minute))
	e.Register(NewRejectRate(0.3, 5*time.Minute))
	return e
}

// Register adds a rule to the engine.
func (e *Engine) Register(r Rule) {
	e.mu.Lock()
	e.rules = append(e.rules, r)
	e.mu.Unlock()
}

// Whitelist returns the whitelist rule for dynamic symbol loading, or nil.
func (e *Engine) Whitelist() *Whitelist {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, r := range e.rules {
		if w, ok := r.(*Whitelist); ok {
			return w
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

// Whitelist rejects orders for symbols not in the allowed list.
// Symbols can be loaded dynamically from broker_symbols via LoadSymbols().
type Whitelist struct {
	mu      sync.RWMutex
	symbols map[string]bool // loaded from broker_symbols at runtime
}

func (r *Whitelist) Name() string { return "whitelist" }

// LoadSymbols replaces the allowed symbol set (e.g. from broker_symbols table).
func (r *Whitelist) LoadSymbols(symbols []string) {
	r.mu.Lock()
	r.symbols = make(map[string]bool, len(symbols))
	for _, s := range symbols {
		r.symbols[s] = true
	}
	r.mu.Unlock()
}

func (r *Whitelist) Check(_ context.Context, req *pb.OrderRequest, _ *AccountState) *pb.RiskCheckResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// If symbols were loaded dynamically, use them.
	if len(r.symbols) > 0 {
		if r.symbols[req.Symbol] {
			return &pb.RiskCheckResult{Approved: true}
		}
		return &pb.RiskCheckResult{Approved: false, Reason: fmt.Sprintf("symbol %s not in whitelist", req.Symbol), RuleId: r.Name()}
	}

	// Fallback: hardcoded major pairs (development without PG).
	static := map[string]bool{
		"EURUSD": true, "EURUSDm": true, "GBPUSD": true, "GBPUSDm": true,
		"USDJPY": true, "USDJPYm": true, "USDCHF": true, "USDCHFm": true,
		"AUDUSD": true, "AUDUSDm": true, "NZDUSD": true, "NZDUSDm": true,
		"USDCAD": true, "USDCADm": true, "XAUUSD": true, "XAUUSDm": true,
	}
	if !static[req.Symbol] {
		return &pb.RiskCheckResult{Approved: false, Reason: fmt.Sprintf("symbol %s not in whitelist", req.Symbol), RuleId: r.Name()}
	}
	return &pb.RiskCheckResult{Approved: true}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
