// Package adminapi — trading-core API sub-component handlers backed by PostgreSQL.
package adminapi

import (
	"context"
	"fmt"

	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/common/db/pg"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Service holds all RPC service implementations for trading-core API layer.
type Service struct {
	pool *pg.Pool
}

// NewService creates a trading-core API service backed by a PG connection pool.
func NewService(pool *pg.Pool) *Service {
	return &Service{pool: pool}
}

// setRLS sets the tenant_id session variable for RLS, extracted from context.
func (s *Service) setRLS(ctx context.Context) error {
	tenantID := auth.TenantFromContext(ctx)
	if tenantID == "" {
		tenantID = "00000000-0000-0000-0000-000000000000" // default tenant for unauthenticated
	}
	return s.pool.SetTenant(ctx, tenantID)
}

// -- BrokerService --

func (s *Service) CreateBroker(ctx context.Context, req *pb.CreateBrokerRequest) (*pb.Broker, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	b := &pb.Broker{}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO brokers (tenant_id, code, name, platform, mtapi_endpoint, default_server)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, tenant_id, code, name, platform, mtapi_endpoint, default_server
	`, req.TenantId, req.Code, req.Name, req.Platform, req.MtapiEndpoint, "").Scan(
		&b.Id, &b.TenantId, &b.Code, &b.Name, &b.Platform, &b.MtapiEndpoint, &b.DefaultServer,
	)
	if err != nil {
		return nil, fmt.Errorf("create broker: %w", err)
	}
	return b, nil
}

func (s *Service) GetBroker(ctx context.Context, req *pb.GetBrokerRequest) (*pb.Broker, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	b := &pb.Broker{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, code, name, platform, mtapi_endpoint, COALESCE(default_server,'')
		FROM brokers WHERE id = $1
	`, req.Id).Scan(&b.Id, &b.TenantId, &b.Code, &b.Name, &b.Platform, &b.MtapiEndpoint, &b.DefaultServer)
	if err != nil {
		return nil, fmt.Errorf("get broker: %w", err)
	}
	return b, nil
}

func (s *Service) ListBrokers(ctx context.Context, req *pb.ListBrokersRequest) (*pb.ListBrokersResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, code, name, platform, mtapi_endpoint, COALESCE(default_server,'')
		FROM brokers WHERE tenant_id = $1 ORDER BY code
	`, req.TenantId)
	if err != nil {
		return nil, fmt.Errorf("list brokers: %w", err)
	}
	defer rows.Close()

	var brokers []*pb.Broker
	for rows.Next() {
		b := &pb.Broker{}
		if err := rows.Scan(&b.Id, &b.TenantId, &b.Code, &b.Name, &b.Platform, &b.MtapiEndpoint, &b.DefaultServer); err != nil {
			return nil, fmt.Errorf("scan broker: %w", err)
		}
		brokers = append(brokers, b)
	}
	return &pb.ListBrokersResponse{Brokers: brokers}, rows.Err()
}

func (s *Service) UpdateBroker(ctx context.Context, req *pb.Broker) (*pb.Broker, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	b := &pb.Broker{}
	err := s.pool.QueryRow(ctx, `
		UPDATE brokers SET code=$1, name=$2, platform=$3, mtapi_endpoint=$4, default_server=$5
		WHERE id = $6
		RETURNING id, tenant_id, code, name, platform, mtapi_endpoint, COALESCE(default_server,'')
	`, req.Code, req.Name, req.Platform, req.MtapiEndpoint, req.DefaultServer, req.Id).Scan(
		&b.Id, &b.TenantId, &b.Code, &b.Name, &b.Platform, &b.MtapiEndpoint, &b.DefaultServer,
	)
	if err != nil {
		return nil, fmt.Errorf("update broker: %w", err)
	}
	return b, nil
}

func (s *Service) DeleteBroker(ctx context.Context, req *pb.DeleteBrokerRequest) (*pb.DeleteBrokerResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM brokers WHERE id = $1`, req.Id)
	if err != nil {
		return nil, fmt.Errorf("delete broker: %w", err)
	}
	return &pb.DeleteBrokerResponse{}, nil
}

// -- AccountService --

func (s *Service) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.Account, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	a := &pb.Account{}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO accounts (tenant_id, broker_id, login, password, server, account_type, currency, leverage)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, broker_id, login, server, account_type, currency, leverage
	`, req.TenantId, req.BrokerId, req.Login, req.Password, req.Server, "demo", "USD", 100).Scan(
		&a.Id, &a.TenantId, &a.BrokerId, &a.Login, &a.Server, &a.AccountType, &a.Currency, &a.Leverage,
	)
	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}
	return a, nil
}

func (s *Service) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.Account, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	a := &pb.Account{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, broker_id, login, server, account_type, currency, leverage
		FROM accounts WHERE id = $1
	`, req.Id).Scan(&a.Id, &a.TenantId, &a.BrokerId, &a.Login, &a.Server, &a.AccountType, &a.Currency, &a.Leverage)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	return a, nil
}

func (s *Service) ListAccounts(ctx context.Context, req *pb.ListAccountsRequest) (*pb.ListAccountsResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, broker_id, login, server, account_type, currency, leverage
		FROM accounts WHERE tenant_id = $1 ORDER BY login
	`, req.TenantId)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*pb.Account
	for rows.Next() {
		a := &pb.Account{}
		if err := rows.Scan(&a.Id, &a.TenantId, &a.BrokerId, &a.Login, &a.Server, &a.AccountType, &a.Currency, &a.Leverage); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, a)
	}
	return &pb.ListAccountsResponse{Accounts: accounts}, rows.Err()
}

func (s *Service) UpdateAccount(ctx context.Context, req *pb.Account) (*pb.Account, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	a := &pb.Account{}
	err := s.pool.QueryRow(ctx, `
		UPDATE accounts SET login=$1, server=$2, account_type=$3, currency=$4, leverage=$5
		WHERE id = $6
		RETURNING id, tenant_id, broker_id, login, server, account_type, currency, leverage
	`, req.Login, req.Server, req.AccountType, req.Currency, req.Leverage, req.Id).Scan(
		&a.Id, &a.TenantId, &a.BrokerId, &a.Login, &a.Server, &a.AccountType, &a.Currency, &a.Leverage,
	)
	if err != nil {
		return nil, fmt.Errorf("update account: %w", err)
	}
	return a, nil
}

func (s *Service) DeleteAccount(ctx context.Context, req *pb.DeleteAccountRequest) (*pb.DeleteAccountResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM accounts WHERE id = $1`, req.Id)
	if err != nil {
		return nil, fmt.Errorf("delete account: %w", err)
	}
	return &pb.DeleteAccountResponse{}, nil
}

// -- StrategyService --

func (s *Service) CreateStrategy(ctx context.Context, req *pb.CreateStrategyRequest) (*pb.Strategy, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	st := &pb.Strategy{}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO strategies (tenant_id, name, description, spec, status)
		VALUES ($1, $2, $3, $4, 'draft')
		RETURNING id, tenant_id, name, description, spec::text, status
	`, req.TenantId, req.Name, req.Description, req.SpecJson).Scan(
		&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("create strategy: %w", err)
	}
	return st, nil
}

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

func (s *Service) ListStrategies(ctx context.Context, req *pb.ListStrategiesRequest) (*pb.ListStrategiesResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, COALESCE(description,''), spec::text, status
		FROM strategies WHERE tenant_id = $1 ORDER BY name
	`, req.TenantId)
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

func (s *Service) DeployStrategy(ctx context.Context, req *pb.DeployStrategyRequest) (*pb.Strategy, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	st := &pb.Strategy{}
	err := s.pool.QueryRow(ctx, `
		UPDATE strategies SET status = 'deployed'
		WHERE id = $1
		RETURNING id, tenant_id, name, COALESCE(description,''), spec::text, status
	`, req.Id).Scan(&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status)
	if err != nil {
		return nil, fmt.Errorf("deploy strategy: %w", err)
	}
	return st, nil
}

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
