package adminapi

import (
	"context"
	"os"
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/common/db/pg"
)

func pgDSN() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}
	return "postgres://alfq:WvFId2cgoVQh8eZxTMuYlQyoq6g7Ba4btqnbAuvUCDU@localhost:5432/alfq?sslmode=disable"
}

func connectPG(t *testing.T) *pg.Pool {
	t.Helper()
	pool, err := pg.Connect(context.Background(), pgDSN())
	if err != nil {
		t.Skipf("PG not available, skipping integration test: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestQ1IntegrationGetSystemSettings(t *testing.T) {
	pool := connectPG(t)
	svc := NewService(pool)
	resp, err := svc.GetSystemSettings(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetSystemSettings: %v", err)
	}
	if len(resp.Settings) == 0 {
		t.Fatal("expected non-empty settings")
	}
	t.Logf("found %d system settings", len(resp.Settings))
}

func TestQ1IntegrationSymbolResolver(t *testing.T) {
	pool := connectPG(t)
	resolver := NewSymbolResolver(pool.Pool)
	info, valid, err := resolver.ResolveCanonical(context.Background(),
		"51b8fe22-1561-4027-802d-32af80d17f6d", "EURUSD")
	if err != nil {
		t.Fatalf("ResolveCanonical: %v", err)
	}
	if !valid {
		t.Fatal("EURUSD should be valid")
	}
	t.Logf("EURUSD → %s (trade_mode=%d)", info.SymbolRaw, info.TradeMode)
}

func TestQ1IntegrationSymbolResolverList(t *testing.T) {
	pool := connectPG(t)
	resolver := NewSymbolResolver(pool.Pool)
	symbols, err := resolver.ListSupportedCanonicals(context.Background(),
		"51b8fe22-1561-4027-802d-32af80d17f6d")
	if err != nil {
		t.Fatalf("ListSupportedCanonicals: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatal("expected non-empty symbol list")
	}
	t.Logf("found %d tradable symbols", len(symbols))
}

func TestQ1IntegrationUpdateSystemSetting(t *testing.T) {
	pool := connectPG(t)
	svc := NewService(pool)
	ctx := auth.WithUser(context.Background(), "00000000-0000-0000-0000-000000000001")
	_, err := svc.UpdateSystemSetting(ctx, &pb.UpdateSystemSettingRequest{
		Key:   "test_integration_q1_key",
		Value: "test_value",
	})
	if err != nil {
		t.Fatalf("UpdateSystemSetting: %v", err)
	}
	pool.Exec(context.Background(), "DELETE FROM system_settings WHERE key='test_integration_q1_key'")
}
