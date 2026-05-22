// Package oms/repo — risk event persistence for audit and promotion gate.
package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/db/pg"
)

// RiskEventRepo persists risk rejection events to PG.
type RiskEventRepo struct {
	pool *pg.Pool
}

// NewRiskEventRepo creates a risk event repository.
func NewRiskEventRepo(pool *pg.Pool) *RiskEventRepo {
	return &RiskEventRepo{pool: pool}
}

// Write persists a risk rejection event with severity classification.
func (r *RiskEventRepo) Write(ctx context.Context, tenantID, accountID, strategyID, ruleID, reason, severity string, orderReq *pb.OrderRequest) error {
	orderJSON, _ := json.Marshal(orderReq)
	_, err := r.pool.Exec(ctx,
		`INSERT INTO risk_events (tenant_id, account_id, strategy_id, event_type, rule_id, reason, severity, order_request_json, ts_unix_ms)
		 VALUES ($1, $2, $3, 'risk_rejected', $4, $5, $6, $7, $8)`,
		tenantID, accountID, strategyID, ruleID, reason, severity, orderJSON, time.Now().UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("risk events insert: %w", err)
	}
	return nil
}
