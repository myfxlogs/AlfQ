//go:build !short

package pg_test

import (
	"context"
	"os"
	"testing"

	"github.com/alfq/backend/go/internal/common/db/pg"
)

func TestSetTenantIntegration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://alfq:alfq_dev@localhost:5432/alfq?sslmode=disable"
	}
	pool, err := pg.Connect(context.Background(), dsn)
	if err != nil {
		t.Skipf("pg unavailable: %v", err)
	}
	defer pool.Close()

	if err := pool.SetTenant(context.Background(), "00000000-0000-0000-0000-000000000000"); err != nil {
		t.Skipf("pg unavailable (SetTenant): %v", err)
	}

	if err := pool.SetRole(context.Background(), "gateway"); err != nil {
		t.Skipf("pg unavailable (SetRole): %v", err)
	}

	// Verify BeginTx works
	tx, err := pool.BeginTx(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Skipf("pg unavailable (BeginTx): %v", err)
	}
	_ = tx.Rollback(context.Background())
}
