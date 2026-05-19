// Package strategysvc — capital allocation module.
package strategysvc

import (
	"sync"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Allocator manages capital distribution across strategies per account.
type Allocator struct {
	mu       sync.RWMutex
	accounts map[string]*AccountAllocation // account_id → allocation
}

// AccountAllocation represents capital allocation for an account.
type AccountAllocation struct {
	AccountID   string
	TotalEquity float64
	Strategies  map[string]*StrategyAllocation // strategy_id → allocation
}

// StrategyAllocation represents a single strategy's capital allocation.
type StrategyAllocation struct {
	StrategyID  string
	AllocPct    float64 // percentage of total equity (0.0 - 1.0)
	MaxQty      float64 // maximum position size
	MaxDrawdown float64 // max allowed drawdown for this strategy
	Enabled     bool
}

// NewAllocator creates a capital allocator.
func NewAllocator() *Allocator {
	return &Allocator{
		accounts: make(map[string]*AccountAllocation),
	}
}

// SetAccount configures the total equity for an account.
func (a *Allocator) SetAccount(accountID string, equity float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.accounts[accountID]; !ok {
		a.accounts[accountID] = &AccountAllocation{
			AccountID:   accountID,
			TotalEquity: equity,
			Strategies:  make(map[string]*StrategyAllocation),
		}
	} else {
		a.accounts[accountID].TotalEquity = equity
	}
}

// AddStrategy registers a strategy with its allocation percentage.
func (a *Allocator) AddStrategy(accountID, strategyID string, allocPct, maxQty, maxDrawdown float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	acc, ok := a.accounts[accountID]
	if !ok {
		acc = &AccountAllocation{AccountID: accountID, Strategies: make(map[string]*StrategyAllocation)}
		a.accounts[accountID] = acc
	}
	acc.Strategies[strategyID] = &StrategyAllocation{
		StrategyID:  strategyID,
		AllocPct:    allocPct,
		MaxQty:      maxQty,
		MaxDrawdown: maxDrawdown,
		Enabled:     true,
	}
}

// RemoveStrategy disables a strategy allocation.
func (a *Allocator) RemoveStrategy(accountID, strategyID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if acc, ok := a.accounts[accountID]; ok {
		if sa, ok := acc.Strategies[strategyID]; ok {
			sa.Enabled = false
		}
	}
}

// MaxOrderSize returns the maximum order size for a strategy, considering allocation.
func (a *Allocator) MaxOrderSize(accountID, strategyID string) float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	acc, ok := a.accounts[accountID]
	if !ok {
		return 0
	}
	sa, ok := acc.Strategies[strategyID]
	if !ok || !sa.Enabled {
		return 0
	}
	return sa.MaxQty
}

// ValidateOrder checks that an order request respects capital allocation limits.
func (a *Allocator) ValidateOrder(req *pb.OrderRequest) *pb.RiskCheckResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	acc, ok := a.accounts[req.AccountId]
	if !ok {
		return &pb.RiskCheckResult{Approved: true} // no allocation configured, allow
	}
	sa, ok := acc.Strategies[req.StrategyId]
	if !ok {
		return &pb.RiskCheckResult{Approved: false, Reason: "strategy not registered for account", RuleId: "capital_alloc"}
	}
	if !sa.Enabled {
		return &pb.RiskCheckResult{Approved: false, Reason: "strategy disabled", RuleId: "capital_alloc"}
	}
	if req.Qty > sa.MaxQty {
		return &pb.RiskCheckResult{Approved: false, Reason: "qty exceeds strategy max allocation", RuleId: "capital_alloc"}
	}
	return &pb.RiskCheckResult{Approved: true}
}

// Summary returns account allocation summary.
func (a *Allocator) Summary(accountID string) *AccountAllocation {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.accounts[accountID]
}
