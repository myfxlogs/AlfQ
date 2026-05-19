// Package adminapi — AccountService RPC handler implementations.
package adminapi

import (
	"context"
	"fmt"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

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
	tenantID := effectiveTenantID(ctx, req.TenantId)
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, broker_id, login, server, account_type, currency, leverage
		FROM accounts WHERE tenant_id = $1 ORDER BY login
	`, tenantID)
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
