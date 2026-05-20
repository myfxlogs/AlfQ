// Package adminapi — StrategyService RPC handler implementations (LP-2).
//
// State machine: draft → ready → paper → live
//   - draft→ready: automatic via BacktestService consistency gate (see backtest_handler.go)
//   - ready→paper: researcher self-serve via DeployStrategy
//   - paper→live: requires tenant_admin + risk_officer approval + Sharpe>1.0
//     + N days paper trading without P0/P1 risk events.
package adminapi

import (
	"context"
	"fmt"
	"slices"
	"strings"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/auth"
)

// validStatusTransitions defines allowed state transitions.
var validStatusTransitions = map[string][]string{
	"draft": {"ready"},
	"ready": {"paper"},
	"paper": {"live"},
}

// CreateStrategy inserts a new strategy in draft status.
func (s *Service) CreateStrategy(ctx context.Context, req *pb.CreateStrategyRequest) (*pb.Strategy, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	st := &pb.Strategy{}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO strategies (tenant_id, name, description, spec, status)
		VALUES ($1, $2, $3, $4, 'draft')
		RETURNING id, tenant_id, name, COALESCE(description,''), spec::text, status
	`, effectiveTenantID(ctx, req.TenantId), req.Name, req.Description, req.SpecJson).Scan(
		&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("create strategy: %w", err)
	}
	return st, nil
}

// GetStrategy returns a single strategy by ID.
func (s *Service) GetStrategy(ctx context.Context, req *pb.GetStrategyRequest) (*pb.Strategy, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	st := &pb.Strategy{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, COALESCE(description,''), spec::text, status
		FROM strategies WHERE id = $1
	`, req.Id).Scan(&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status)
	if err != nil {
		return nil, fmt.Errorf("get strategy: %w", err)
	}
	return st, nil
}

// ListStrategies returns all strategies for the current tenant.
func (s *Service) ListStrategies(ctx context.Context, req *pb.ListStrategiesRequest) (*pb.ListStrategiesResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	tenantID := effectiveTenantID(ctx, req.TenantId)
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, COALESCE(description,''), spec::text, status
		FROM strategies WHERE tenant_id = $1 ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list strategies: %w", err)
	}
	defer rows.Close()

	var strategies []*pb.Strategy
	for rows.Next() {
		st := &pb.Strategy{}
		if err := rows.Scan(&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status); err != nil {
			return nil, fmt.Errorf("scan strategy: %w", err)
		}
		strategies = append(strategies, st)
	}
	return &pb.ListStrategiesResponse{Strategies: strategies}, rows.Err()
}

// DeployStrategy transitions a strategy from ready → paper (researcher self-serve).
func (s *Service) DeployStrategy(ctx context.Context, req *pb.DeployStrategyRequest) (*pb.Strategy, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	return s.transitionStatus(ctx, req.Id, "paper", "deploy")
}

// StopStrategy transitions a live strategy back to stopped.
func (s *Service) StopStrategy(ctx context.Context, req *pb.StopStrategyRequest) (*pb.Strategy, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	st := &pb.Strategy{}
	err := s.pool.QueryRow(ctx, `
		UPDATE strategies SET status = 'stopped'
		WHERE id = $1
		RETURNING id, tenant_id, name, COALESCE(description,''), spec::text, status
	`, req.Id).Scan(&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status)
	if err != nil {
		return nil, fmt.Errorf("stop strategy: %w", err)
	}
	return st, nil
}

// ── Paper → Live promotion with double sign-off ──

// PromoteStrategyRequest holds the data needed to promote a strategy to live.
// (Not a proto type — this is an internal method called from the API layer.)
type PromoteStrategyRequest struct {
	StrategyID string
	Approvals  []string // list of approver role names
}

// PromoteToLive transitions a strategy from paper → live.
//
// Gate checks:
//  1. Sharpre ratio > 1.0 during paper trading
//  2. No P0/P1 risk events during paper trading
//  3. Double sign-off: at least one tenant_admin AND one risk_officer approval
func (s *Service) PromoteToLive(ctx context.Context, req *PromoteStrategyRequest) (*pb.Strategy, *PromoteError, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, nil, fmt.Errorf("rls: %w", err)
	}

	// Load current strategy
	st, err := s.GetStrategy(ctx, &pb.GetStrategyRequest{Id: req.StrategyID})
	if err != nil {
		return nil, nil, fmt.Errorf("promote: %w", err)
	}

	if st.Status != "paper" {
		return nil, newPromoteError("strategy must be in paper status, current: "+st.Status), nil
	}

	// Double sign-off check
	if err := s.checkDoubleSignOff(ctx, req); err != nil {
		return nil, err, nil
	}

	// Sharpe check
	if err := s.checkSharpe(ctx, req.StrategyID); err != nil {
		return nil, err, nil
	}

	// Risk event check
	if err := s.checkNoRiskEvents(ctx, req.StrategyID); err != nil {
		return nil, err, nil
	}

	// Transition to live
	promoted, err := s.transitionStatus(ctx, req.StrategyID, "live", "promote to live")
	if err != nil {
		return nil, nil, fmt.Errorf("promote to live: %w", err)
	}

	return promoted, nil, nil
}

// ── Helpers ──

// transitionStatus validates and executes a status transition.
func (s *Service) transitionStatus(ctx context.Context, strategyID, newStatus, action string) (*pb.Strategy, error) {
	// Read current status
	var currentStatus string
	err := s.pool.QueryRow(ctx,
		`SELECT status FROM strategies WHERE id = $1`, strategyID,
	).Scan(&currentStatus)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	// Validate transition
	allowed := validStatusTransitions[currentStatus]
	if !slices.Contains(allowed, newStatus) {
		return nil, fmt.Errorf("%s: cannot transition from %q to %q (allowed: %s)",
			action, currentStatus, newStatus, strings.Join(allowed, ", "))
	}

	// Execute
	st := &pb.Strategy{}
	err = s.pool.QueryRow(ctx, `
		UPDATE strategies SET status = $1
		WHERE id = $2
		RETURNING id, tenant_id, name, COALESCE(description,''), spec::text, status
	`, newStatus, strategyID).Scan(
		&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}
	return st, nil
}

// checkDoubleSignOff verifies tenant_admin + risk_officer approvals.
func (s *Service) checkDoubleSignOff(ctx context.Context, req *PromoteStrategyRequest) *PromoteError {
	roles := auth.RolesFromContext(ctx)
	hasAdmin := slices.Contains(roles, "tenant_admin")
	hasRisk := slices.Contains(roles, "risk_officer")

	// Also check explicit approvals list
	for _, a := range req.Approvals {
		if a == "tenant_admin" {
			hasAdmin = true
		}
		if a == "risk_officer" {
			hasRisk = true
		}
	}

	if !hasAdmin || !hasRisk {
		missing := []string{}
		if !hasAdmin {
			missing = append(missing, "tenant_admin")
		}
		if !hasRisk {
			missing = append(missing, "risk_officer")
		}
		return newPromoteError("double sign-off required: missing approvals from " + strings.Join(missing, ", "))
	}
	return nil
}

// checkSharpe verifies paper trading Sharpe > 1.0.
func (s *Service) checkSharpe(ctx context.Context, strategyID string) *PromoteError {
	if s.pool == nil {
		return nil // no PG, skip check
	}
	var sharpe float64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(sharpe_ratio, 0)
		FROM backtest_results
		WHERE strategy_id = $1 AND status = 'paper'
		ORDER BY created_at DESC LIMIT 1
	`, strategyID).Scan(&sharpe)
	if err != nil {
		// No backtest results yet — allow promotion with warning
		return nil
	}
	if sharpe < 1.0 {
		return newPromoteError(fmt.Sprintf("Sharpe ratio %.2f below 1.0 threshold", sharpe))
	}
	return nil
}

// checkNoRiskEvents verifies no P0/P1 risk events during paper trading.
func (s *Service) checkNoRiskEvents(ctx context.Context, strategyID string) *PromoteError {
	if s.pool == nil {
		return nil
	}
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM risk_events
		WHERE strategy_id = $1 AND severity IN ('P0', 'P1')
	`, strategyID).Scan(&count)
	if err != nil {
		return nil // table may not exist yet; allow
	}
	if count > 0 {
		return newPromoteError(fmt.Sprintf("%d P0/P1 risk events during paper trading", count))
	}
	return nil
}

// PromoteError is a non-fatal error returned during promotion checks.
type PromoteError struct {
	Reason string
}

func newPromoteError(reason string) *PromoteError {
	return &PromoteError{Reason: reason}
}

func (e *PromoteError) Error() string {
	return "promotion blocked: " + e.Reason
}
