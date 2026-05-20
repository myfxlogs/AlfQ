//go:build !short

package adminapi

import (
	"connectrpc.com/connect"
	"context"
	"os"
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/common/db/pg"
)

const testTenantID = "00000000-0000-0000-0000-000000000099"

// testCtx returns a context with the test tenant injected (required by setRLS).
func testCtx() context.Context {
	return auth.WithTenant(context.Background(), testTenantID)
}

func setupDB(t *testing.T) *pg.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://alfq:alfq_dev@localhost:5432/alfq?sslmode=disable"
	}
	pool, err := pg.Connect(context.Background(), dsn)
	if err != nil {
		t.Skipf("pg unavailable: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS brokers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			code TEXT NOT NULL,
			name TEXT NOT NULL,
			platform TEXT NOT NULL DEFAULT 'mt5',
			mtapi_endpoint TEXT NOT NULL DEFAULT '',
			default_server TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS accounts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			broker_id UUID NOT NULL,
			login TEXT NOT NULL,
			password TEXT NOT NULL DEFAULT '',
			server TEXT NOT NULL DEFAULT '',
			account_type TEXT NOT NULL DEFAULT 'demo',
			currency TEXT NOT NULL DEFAULT 'USD',
			leverage INTEGER NOT NULL DEFAULT 100
		)`,
		`CREATE TABLE IF NOT EXISTS strategies (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			spec JSONB NOT NULL DEFAULT '{}',
			status TEXT NOT NULL DEFAULT 'draft'
		)`,
	} {
		if _, err := pool.Exec(context.Background(), stmt); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	// Clean before each test
	pool.Exec(context.Background(), "DELETE FROM accounts")
	pool.Exec(context.Background(), "DELETE FROM strategies")
	pool.Exec(context.Background(), "DELETE FROM brokers")
	return pool
}

func TestIntegrationBrokerCRUD(t *testing.T) {
	pool := setupDB(t)
	defer pool.Close()

	svc := NewService(pool)
	tid := testTenantID

	// Create
	b, err := svc.CreateBroker(testCtx(), &pb.CreateBrokerRequest{
		TenantId: tid, Code: "INT-T1", Name: "Integration Broker", Platform: "mt5",
	})
	if err != nil {
		t.Fatalf("CreateBroker: %v", err)
	}

	// List
	resp, err := svc.ListBrokers(testCtx(), &pb.ListBrokersRequest{TenantId: tid})
	if err != nil {
		t.Fatalf("ListBrokers: %v", err)
	}
	if len(resp.Brokers) < 1 {
		t.Fatal("expected at least 1 broker")
	}

	// Get
	got, err := svc.GetBroker(testCtx(), &pb.GetBrokerRequest{Id: b.Id})
	if err != nil {
		t.Fatalf("GetBroker: %v", err)
	}
	if got.Code != "INT-T1" {
		t.Fatalf("Code: %s", got.Code)
	}

	// Update
	upd, err := svc.UpdateBroker(testCtx(), &pb.Broker{
		Id: b.Id, TenantId: b.TenantId, Code: "INT-T2", Name: "Updated", Platform: "mt4",
	})
	if err != nil {
		t.Fatalf("UpdateBroker: %v", err)
	}
	if upd.Name != "Updated" {
		t.Fatalf("Name: %s", upd.Name)
	}

	// Delete
	_, err = svc.DeleteBroker(testCtx(), &pb.DeleteBrokerRequest{Id: b.Id})
	if err != nil {
		t.Fatalf("DeleteBroker: %v", err)
	}
}

func TestIntegrationAccountCRUD(t *testing.T) {
	pool := setupDB(t)
	defer pool.Close()

	svc := NewService(pool)
	tid := testTenantID

	brk, err := svc.CreateBroker(testCtx(), &pb.CreateBrokerRequest{
		TenantId: tid, Code: "INT-ACC-BRK", Name: "Acc Broker", Platform: "mt5",
	})
	if err != nil {
		t.Fatalf("CreateBroker: %v", err)
	}

	a, err := svc.CreateAccount(testCtx(), &pb.CreateAccountRequest{
		TenantId: tid, BrokerId: brk.Id, Login: "99999", Password: "secret",
	})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	resp, err := svc.ListAccounts(testCtx(), &pb.ListAccountsRequest{TenantId: tid})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(resp.Accounts) < 1 {
		t.Fatal("expected at least 1 account")
	}

	got, err := svc.GetAccount(testCtx(), &pb.GetAccountRequest{Id: a.Id})
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got.Login != "99999" {
		t.Fatalf("Login: %s", got.Login)
	}

	upd, err := svc.UpdateAccount(testCtx(), &pb.Account{
		Id: a.Id, TenantId: a.TenantId, BrokerId: a.BrokerId,
		Login: a.Login, Server: "Demo", AccountType: "real", Currency: "EUR", Leverage: 200,
	})
	if err != nil {
		t.Fatalf("UpdateAccount: %v", err)
	}
	if upd.Currency != "EUR" {
		t.Fatalf("Currency: %s", upd.Currency)
	}

	_, err = svc.DeleteAccount(testCtx(), &pb.DeleteAccountRequest{Id: a.Id})
	if err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
}

func TestIntegrationStrategyLifecycle(t *testing.T) {
	pool := setupDB(t)
	defer pool.Close()

	svc := NewService(pool)
	tid := testTenantID

	st, err := svc.CreateStrategy(testCtx(), &pb.CreateStrategyRequest{
		TenantId: tid, Name: "IntStrat", Description: "integration test", SpecJson: `{"entry":"macd"}`,
	})
	if err != nil {
		t.Fatalf("CreateStrategy: %v", err)
	}

	resp, err := svc.ListStrategies(testCtx(), &pb.ListStrategiesRequest{TenantId: tid})
	if err != nil {
		t.Fatalf("ListStrategies: %v", err)
	}
	if len(resp.Strategies) < 1 {
		t.Fatal("expected at least 1 strategy")
	}

	got, err := svc.GetStrategy(testCtx(), &pb.GetStrategyRequest{Id: st.Id})
	if err != nil {
		t.Fatalf("GetStrategy: %v", err)
	}
	if got.Name != "IntStrat" {
		t.Fatalf("Name: %s", got.Name)
	}

	dep, err := svc.DeployStrategy(testCtx(), &pb.DeployStrategyRequest{Id: st.Id})
	if err != nil {
		t.Fatalf("DeployStrategy: %v", err)
	}
	if dep.Status != "deployed" {
		t.Fatalf("Status: %s", dep.Status)
	}

	stopped, err := svc.StopStrategy(testCtx(), &pb.StopStrategyRequest{Id: st.Id})
	if err != nil {
		t.Fatalf("StopStrategy: %v", err)
	}
	if stopped.Status != "stopped" {
		t.Fatalf("Status: %s", stopped.Status)
	}
}

func TestIntegrationAdapterBroker(t *testing.T) {
	pool := setupDB(t)
	defer pool.Close()

	svc := NewService(pool)
	adp := NewAdapter(svc)
	tid := testTenantID

	b, err := adp.CreateBroker(testCtx(), connect.NewRequest(&pb.CreateBrokerRequest{
		TenantId: tid, Code: "ADP-BRK", Name: "Adapter Broker", Platform: "mt5",
	}))
	if err != nil {
		t.Fatalf("adapter CreateBroker: %v", err)
	}

	resp, err := adp.ListBrokers(testCtx(), connect.NewRequest(&pb.ListBrokersRequest{TenantId: tid}))
	if err != nil {
		t.Fatalf("adapter ListBrokers: %v", err)
	}
	if len(resp.Msg.Brokers) < 1 {
		t.Fatal("expected brokers")
	}

	_, err = adp.DeleteBroker(testCtx(), connect.NewRequest(&pb.DeleteBrokerRequest{Id: b.Msg.Id}))
	if err != nil {
		t.Fatalf("adapter DeleteBroker: %v", err)
	}
}

func TestIntegrationAdapterAccount(t *testing.T) {
	pool := setupDB(t)
	defer pool.Close()

	svc := NewService(pool)
	adp := NewAdapter(svc)
	tid := testTenantID

	brk, _ := adp.CreateBroker(testCtx(), connect.NewRequest(&pb.CreateBrokerRequest{
		TenantId: tid, Code: "ADP-ACC", Name: "A", Platform: "mt5",
	}))

	a, err := adp.CreateAccount(testCtx(), connect.NewRequest(&pb.CreateAccountRequest{
		TenantId: tid, BrokerId: brk.Msg.Id, Login: "adp-1", Password: "x",
	}))
	if err != nil {
		t.Fatalf("adapter CreateAccount: %v", err)
	}

	resp, err := adp.ListAccounts(testCtx(), connect.NewRequest(&pb.ListAccountsRequest{TenantId: tid}))
	if err != nil {
		t.Fatalf("adapter ListAccounts: %v", err)
	}
	if len(resp.Msg.Accounts) < 1 {
		t.Fatal("expected accounts")
	}

	_, err = adp.DeleteAccount(testCtx(), connect.NewRequest(&pb.DeleteAccountRequest{Id: a.Msg.Id}))
	if err != nil {
		t.Fatalf("adapter DeleteAccount: %v", err)
	}
}
