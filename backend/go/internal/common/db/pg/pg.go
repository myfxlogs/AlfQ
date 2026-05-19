// Package pg provides PostgreSQL connection pool utilities using pgx.
package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps a pgx connection pool with tenant RLS context support.
type Pool struct {
	*pgxpool.Pool
}

// Connect creates a new connection pool.
func Connect(ctx context.Context, dsn string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pg: parse dsn: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pg: connect: %w", err)
	}
	return &Pool{Pool: pool}, nil
}

// SetTenant sets the tenant_id for RLS context.
// Must be called at the beginning of each session/transaction.
func (p *Pool) SetTenant(ctx context.Context, tenantID string) error {
	_, err := p.Exec(ctx, "SELECT set_config('app.tenant_id', $1, false)", tenantID)
	return err
}

// Close releases the pool.
func (p *Pool) Close() { p.Pool.Close() }
