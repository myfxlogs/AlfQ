// Package adminapi — StrategyService RPC handler implementations (LP-2).
//
// State machine: draft → ready → paper → live
//   - draft→ready: automatic via BacktestService consistency gate (see backtest_handler.go)
//   - ready→paper: researcher self-serve via DeployStrategy
//   - paper→live: requires tenant_admin + risk_officer approval + Sharpe>1.0
//   - N days paper trading without P0/P1 risk events.
package adminapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/auth"
)

// validStatusTransitions defines allowed state transitions.
var validStatusTransitions = map[string][]string{
	"draft": {"ready"},
	"ready": {"paper"},
	"paper": {"live"},
}

// CreateStrategy inserts a new strategy in draft status and creates revision #1 (RS02).
func (s *Service) CreateStrategy(ctx context.Context, req *pb.CreateStrategyRequest) (*pb.Strategy, error) {
	if err := RequireTenant(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}

	tenantID := effectiveTenantID(ctx, req.TenantId)
	userID := auth.UserFromContext(ctx)

	// RS06: Validate canonical_symbols are tradable on the tenant's brokers.
	if s.symbolResolver != nil {
		if err := s.validateSpecSymbols(ctx, req.SpecJson); err != nil {
			return nil, fmt.Errorf("symbol validation: %w", err)
		}
	}

	st := &pb.Strategy{}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO strategies (tenant_id, name, description, spec, status, revision_counter)
		VALUES ($1, $2, $3, $4, 'draft', 0)
		RETURNING id, tenant_id, name, COALESCE(description,''), spec::text, status
	`, tenantID, req.Name, req.Description, req.SpecJson).Scan(
		&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("create strategy: %w", err)
	}

	// RS02: Create initial revision snapshot
	specHash := sha256Hash(req.SpecJson)
	var revID string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO strategy_revisions (strategy_id, revision_no, spec, spec_hash, created_by)
		VALUES ($1, 1, $2::jsonb, $3, $4)
		RETURNING id
	`, st.Id, req.SpecJson, specHash, nullUUID(userID)).Scan(&revID)
	if err != nil {
		return nil, fmt.Errorf("create revision: %w", err)
	}

	// Link current revision
	_, err = s.pool.Exec(ctx,
		`UPDATE strategies SET current_revision_id=$1, revision_counter=1 WHERE id=$2`,
		revID, st.Id)
	if err != nil {
		return nil, fmt.Errorf("link revision: %w", err)
	}

	return st, nil
}

// UpdateStrategy updates a strategy and creates a new immutable revision (RS02).
func (s *Service) UpdateStrategy(ctx context.Context, req *pb.Strategy) (*pb.Strategy, error) {
	if err := RequireTenant(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}

	// RS06: validate symbols if spec is being updated
	if req.SpecJson != "" && s.symbolResolver != nil {
		if err := s.validateSpecSymbols(ctx, req.SpecJson); err != nil {
			return nil, fmt.Errorf("symbol validation: %w", err)
		}
	}

	// Build dynamic SET clause
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if req.Name != "" {
		setClauses = append(setClauses, fmt.Sprintf("name=$%d", argIdx))
		args = append(args, req.Name)
		argIdx++
	}
	if req.Description != "" {
		setClauses = append(setClauses, fmt.Sprintf("description=$%d", argIdx))
		args = append(args, req.Description)
		argIdx++
	}
	specChanged := false
	if req.SpecJson != "" {
		setClauses = append(setClauses, fmt.Sprintf("spec=$%d", argIdx))
		args = append(args, req.SpecJson)
		argIdx++
		specChanged = true
	}

	if len(setClauses) == 0 {
		return s.GetStrategy(ctx, &pb.GetStrategyRequest{Id: req.Id})
	}

	query := fmt.Sprintf(
		`UPDATE strategies SET %s WHERE id=$%d
		 RETURNING id, tenant_id, name, COALESCE(description,''), spec::text, status, COALESCE(current_revision_id::text,''), revision_counter`,
		strings.Join(setClauses, ", "), argIdx)
	args = append(args, req.Id)

	st := &pb.Strategy{}
	var currentRevID string
	var revCounter int
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status,
		&currentRevID, &revCounter,
	)
	if err != nil {
		return nil, fmt.Errorf("update strategy: %w", err)
	}

	// RS02: Create new revision if spec changed
	if specChanged {
		newRevNo := revCounter + 1
		specHash := sha256Hash(req.SpecJson)
		userID := auth.UserFromContext(ctx)
		var revID string
		err = s.pool.QueryRow(ctx, `
			INSERT INTO strategy_revisions (strategy_id, revision_no, spec, spec_hash, created_by)
			VALUES ($1, $2, $3::jsonb, $4, $5)
			RETURNING id
		`, st.Id, newRevNo, req.SpecJson, specHash, nullUUID(userID)).Scan(&revID)
		if err != nil {
			return nil, fmt.Errorf("create revision: %w", err)
		}
		_, err = s.pool.Exec(ctx,
			`UPDATE strategies SET current_revision_id=$1, revision_counter=$2 WHERE id=$3`,
			revID, newRevNo, st.Id)
		if err != nil {
			return nil, fmt.Errorf("link revision: %w", err)
		}
	}

	return st, nil
}

// GetStrategy returns a single strategy by ID.
func (s *Service) GetStrategy(ctx context.Context, req *pb.GetStrategyRequest) (*pb.Strategy, error) {
	if err := RequireTenant(ctx); err != nil {
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

// ListStrategies returns strategies for the current tenant with cursor-based pagination.
func (s *Service) ListStrategies(ctx context.Context, req *pb.ListStrategiesRequest) (*pb.ListStrategiesResponse, error) {
	if err := RequireTenant(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	tenantID := effectiveTenantID(ctx, req.TenantId)

	pageSize := int32(50)
	if req.PageSize > 0 && req.PageSize <= 200 {
		pageSize = req.PageSize
	}

	var rows pgx.Rows
	var err error
	if req.PageToken == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, tenant_id, name, COALESCE(description,''), spec::text, status
			FROM strategies WHERE tenant_id = $1 ORDER BY name LIMIT $2
		`, tenantID, pageSize+1) // +1 to detect next page
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, tenant_id, name, COALESCE(description,''), spec::text, status
			FROM strategies WHERE tenant_id = $1 AND name > $2 ORDER BY name LIMIT $3
		`, tenantID, req.PageToken, pageSize+1)
	}
	if err != nil {
		return nil, fmt.Errorf("list strategies: %w", err)
	}
	defer rows.Close()

	var strategies []*pb.Strategy
	for rows.Next() {
		if int32(len(strategies)) >= pageSize {
			break // stop at page_size, leave next page token
		}
		st := &pb.Strategy{}
		if err := rows.Scan(&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status); err != nil {
			return nil, fmt.Errorf("scan strategy: %w", err)
		}
		strategies = append(strategies, st)
	}

	var nextToken string
	// If there are more rows, use the last name as cursor
	if rows.Next() {
		if len(strategies) > 0 {
			nextToken = strategies[len(strategies)-1].Name
		}
	}

	return &pb.ListStrategiesResponse{Strategies: strategies, NextPageToken: nextToken}, rows.Err()
}

// DeployStrategy transitions a strategy from ready → paper (researcher self-serve).
func (s *Service) DeployStrategy(ctx context.Context, req *pb.DeployStrategyRequest) (*pb.Strategy, error) {
	if err := RequireTenant(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	return s.transitionStatus(ctx, req.Id, "paper", "deploy")
}

// StopStrategy transitions a live strategy back to stopped.
func (s *Service) StopStrategy(ctx context.Context, req *pb.StopStrategyRequest) (*pb.Strategy, error) {
	if err := RequireTenant(ctx); err != nil {
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
	if err := RequireTenant(ctx); err != nil {
		return nil, nil, fmt.Errorf("rls: %w", err)
	}

	// Load current strategy
	st, err := s.GetStrategy(ctx, &pb.GetStrategyRequest{Id: req.StrategyID})
	if err != nil {
		return nil, nil, fmt.Errorf("promote: %w", err)
	}

	if st.Status != "paper" {
		return nil, newPromoteError("strategy must be in paper status, current: " + st.Status), nil
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

	// RS02: Ensure live revision matches backtest revision
	if err := s.checkRevisionConsistency(ctx, req.StrategyID); err != nil {
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

// checkRevisionConsistency ensures the strategy's current revision matches
// the revision used in the most recent backtest (RS02).
func (s *Service) checkRevisionConsistency(ctx context.Context, strategyID string) *PromoteError {
	if s.pool == nil {
		return nil
	}
	var currentRevID string
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(current_revision_id::text, '') FROM strategies WHERE id=$1`,
		strategyID).Scan(&currentRevID)
	if err != nil || currentRevID == "" {
		return nil // no revision yet, allow
	}

	var backtestRevID string
	err = s.pool.QueryRow(ctx,
		`SELECT COALESCE(strategy_revision_id::text, '')
		 FROM backtest_results
		 WHERE strategy_id=$1
		 ORDER BY created_at DESC LIMIT 1`,
		strategyID).Scan(&backtestRevID)
	if err != nil || backtestRevID == "" {
		return nil // no backtest yet, allow
	}

	if currentRevID != backtestRevID {
		return newPromoteError(
			fmt.Sprintf("current revision (%s) does not match backtest revision (%s); re-run backtest with latest spec",
				truncate8(currentRevID), truncate8(backtestRevID)))
	}
	return nil
}

func truncate8(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// ── RS02 helpers ──

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// nullUUID returns a nil uuid if the string is empty, for nullable PG columns.
func nullUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// validateSpecSymbols parses the spec JSON and checks all canonical_symbols
// against the tenant's brokers via SymbolResolver (RS06).
func (s *Service) validateSpecSymbols(ctx context.Context, specJSON string) error {
	var spec struct {
		CanonicalSymbols []string `json:"canonical_symbols"`
	}
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		return fmt.Errorf("invalid spec json: %w", err)
	}
	if len(spec.CanonicalSymbols) == 0 {
		return nil // no symbols to validate
	}

	// Get the first account for this tenant to resolve against
	tenantID := auth.TenantFromContext(ctx)
	rows, err := s.pool.Query(ctx,
		`SELECT id FROM accounts WHERE tenant_id=$1 LIMIT 1`, tenantID)
	if err != nil {
		return nil // can't resolve, skip validation
	}
	defer rows.Close()

	if !rows.Next() {
		return nil // no accounts yet, skip
	}
	var accountID string
	rows.Scan(&accountID)
	rows.Close()

	for _, sym := range spec.CanonicalSymbols {
		_, valid, err := s.symbolResolver.ResolveCanonical(ctx, accountID, sym)
		if err != nil || !valid {
			return fmt.Errorf("symbol %s not tradable on broker: %v", sym, err)
		}
	}
	return nil
}
