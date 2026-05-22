// Package pg provides PostgreSQL connection pool utilities using pgx.
package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
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
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		// Reset tenant_id on every connection acquisition to prevent cross-tenant leakage.
		if _, err := conn.Exec(ctx, "SELECT set_config('app.tenant_id', '', true)"); err != nil {
			return fmt.Errorf("pg: afterconnect reset tenant: %w", err)
		}
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pg: connect: %w", err)
	}
	return &Pool{Pool: pool}, nil
}

// SetTenant sets the tenant_id for RLS context.
// Uses LOCAL scope so it is automatically discarded at transaction end.
func (p *Pool) SetTenant(ctx context.Context, tenantID string) error {
	_, err := p.Exec(ctx, "SELECT set_config('app.tenant_id', $1, false)", tenantID)
	return err
}

// SetRole sets an application role for the current session (e.g. 'gateway').
// Used by infrastructure services that need cross-tenant access for metadata tables.
func (p *Pool) SetRole(ctx context.Context, role string) error {
	_, err := p.Exec(ctx, "SELECT set_config('app.role', $1, false)", role)
	return err
}

// Close releases the pool.
func (p *Pool) Close() {
	if p != nil && p.Pool != nil {
		p.Pool.Close()
	}
}

// BeginTx starts a new transaction and sets the tenant for RLS.
// Callers must call Commit or Rollback when done.
func (p *Pool) BeginTx(ctx context.Context, tenantID string) (pgx.Tx, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("pg: begin tx: %w", err)
	}
	if tenantID != "" {
		_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID)
	}
	return tx, err
}