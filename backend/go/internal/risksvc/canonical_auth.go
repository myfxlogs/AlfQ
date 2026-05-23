// Package risksvc — CanonicalAuth rule.
// Replaces the legacy Whitelist with three-gate authorization:
// Gate-1: strategy_symbols check (not_in_strategy_whitelist)
// Gate-2: tenant_canonical_whitelist check (tenant_not_authorized)
// Gate-3: broker_symbols resolution (symbol_not_on_broker) — handled by OMS executor
package risksvc

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CanonicalAuth enforces strategy-level and tenant-level canonical symbol authorization.
// It replaces the legacy Whitelist (global map of all broker symbols).
type CanonicalAuth struct {
	pool  *pgxpool.Pool
	cache sync.Map // (strategyID + ":" + canonical) → cached result
}

// NewCanonicalAuth creates a CanonicalAuth rule backed by PG.
func NewCanonicalAuth(pool *pgxpool.Pool) *CanonicalAuth {
	return &CanonicalAuth{pool: pool}
}

func (r *CanonicalAuth) Name() string { return "canonical_auth" }

// Check enforces Gates 1 and 2 of the three-gate authorization chain.
// Gate-1: strategy_symbols — does this strategy allow this canonical?
// Gate-2: tenant_canonical_whitelist — does this tenant allow this canonical?
// Gate-3: broker_symbols resolution — handled by OMS executor after risk passes.
func (r *CanonicalAuth) Check(ctx context.Context, req *pb.OrderRequest, _ *AccountState) *pb.RiskCheckResult {
	// No PG: allow all (development mode)
	if r.pool == nil {
		return &pb.RiskCheckResult{Approved: true}
	}

	canonical := req.Symbol
	strategyID := req.StrategyId
	tenantID := req.TenantId

	// Cache key
	cacheKey := strategyID + ":" + canonical

	// Check cache
	if v, ok := r.cache.Load(cacheKey); ok {
		if allowed, ok := v.(bool); ok && allowed {
			return &pb.RiskCheckResult{Approved: true}
		}
		return &pb.RiskCheckResult{
			Approved: false,
			Reason:   fmt.Sprintf("canonical %s not authorized for strategy %s (cached)", canonical, strategyID),
			RuleId:   r.Name(),
		}
	}

	// Gate-1: strategy_symbols check
	var gate1 bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM strategy_symbols ss
			WHERE ss.strategy_id = $1 AND ss.canonical = $2 AND ss.enabled = true
		)`,
		strategyID, canonical,
	).Scan(&gate1)
	if err != nil {
		// DB error → allow (fail-open for availability; reject happens downstream on broker)
		return &pb.RiskCheckResult{Approved: true}
	}
	if !gate1 {
		r.cache.Store(cacheKey, false)
		return &pb.RiskCheckResult{
			Approved: false,
			Reason:   fmt.Sprintf("not_in_strategy_whitelist: %s not in strategy_symbols for strategy %s", canonical, strategyID),
			RuleId:   "canonical_auth.gate1",
		}
	}

	// Gate-2: tenant_canonical_whitelist check
	var gate2 bool
	err = r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM tenant_canonical_whitelist tcw
			WHERE tcw.tenant_id = $1 AND tcw.canonical = $2 AND tcw.enabled = true
		)`,
		tenantID, canonical,
	).Scan(&gate2)
	if err != nil {
		return &pb.RiskCheckResult{Approved: true}
	}
	if !gate2 {
		r.cache.Store(cacheKey, false)
		return &pb.RiskCheckResult{
			Approved: false,
			Reason:   fmt.Sprintf("tenant_not_authorized: canonical %s not in tenant_canonical_whitelist for tenant %s", canonical, tenantID),
			RuleId:   "canonical_auth.gate2",
		}
	}

	// Both gates passed
	r.cache.Store(cacheKey, true)
	return &pb.RiskCheckResult{Approved: true}
}

// StartNotifyListener subscribes to PG NOTIFY on strategy_symbols changes
// and invalidates the cache in real time (M6 hot-reload).
// Runs in a background goroutine; call once per CanonicalAuth instance.
func (r *CanonicalAuth) StartNotifyListener(ctx context.Context) {
	if r.pool == nil {
		return
	}
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return
	}
	if _, err := conn.Exec(ctx, "LISTEN strategy_symbols"); err != nil {
		conn.Release()
		return
	}
	go func() {
		defer conn.Release()
		for {
			notif, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				return
			}
			// Payload is the strategy_id
			strategyID := notif.Payload
			r.InvalidateCache(strategyID, "")
		}
	}()
}

// InvalidateCache clears the cache for a specific strategy+canonical combo.
// Called when strategy_symbols changes (via NOTIFY).
func (r *CanonicalAuth) InvalidateCache(strategyID, canonical string) {
	if canonical != "" {
		r.cache.Delete(strategyID + ":" + canonical)
	} else {
		// Invalidate all entries for this strategy
		prefix := strategyID + ":"
		r.cache.Range(func(key, _ interface{}) bool {
			if k, ok := key.(string); ok && len(k) > len(prefix) && k[:len(prefix)] == prefix {
				r.cache.Delete(k)
			}
			return true
		})
	}
}
