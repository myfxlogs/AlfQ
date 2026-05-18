// Package oms/repo — PG position repository.
package repo

import (
	"context"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/db/pg"
)

// PositionRepo provides PostgreSQL access for positions.
type PositionRepo struct {
	pool *pg.Pool
}

// NewPositionRepo creates a position repository.
func NewPositionRepo(pool *pg.Pool) *PositionRepo { return &PositionRepo{pool: pool} }

// Upsert creates or updates a position row.
func (r *PositionRepo) Upsert(ctx context.Context, pos *pb.Position) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO positions (position_id, tenant_id, account_id, symbol, qty, avg_price)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (account_id, symbol) DO UPDATE SET qty = $5, avg_price = $6`,
		pos.PositionId, pos.TenantId, pos.AccountId, pos.Symbol, pos.Qty, pos.GetAvgPrice().GetValue(),
	)
	return err
}

// FindByAccount returns all positions for an account.
func (r *PositionRepo) FindByAccount(ctx context.Context, accountID string) ([]*pb.Position, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT position_id, tenant_id, account_id, symbol, qty, avg_price FROM positions WHERE account_id = $1`,
		accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*pb.Position
	for rows.Next() {
		var p pb.Position
		if err := rows.Scan(&p.PositionId, &p.TenantId, &p.AccountId, &p.Symbol, &p.Qty, &p.AvgPrice); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, nil
}
