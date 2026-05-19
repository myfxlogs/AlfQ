// Package adminapi — AccountService RPC handler implementations.
package adminapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alfq/backend/go/internal/mdgateway/adapter/mtapi"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"go.uber.org/zap"
)

func (s *Service) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.Account, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}

	// 1. Look up broker to get MT endpoint
	var brokerHost string
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(mtapi_endpoint, '') FROM brokers WHERE id = $1`,
		req.BrokerId,
	).Scan(&brokerHost)
	if err != nil {
		return nil, fmt.Errorf("broker lookup: %w", err)
	}

	// 2. Insert account with connecting status
	a := &pb.Account{}
	now := time.Now()
	err = s.pool.QueryRow(ctx, `
		INSERT INTO accounts (tenant_id, broker_id, login, password, server, account_type,
			status, currency, leverage, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'connecting', 'USD', 100, $7, $7)
		RETURNING id, tenant_id, broker_id, login, server, account_type, currency, leverage,
			status, balance, equity, margin, free_margin, margin_level, profit, profit_percent,
			is_disabled, last_error, alias
	`, req.TenantId, req.BrokerId, req.Login, req.Password, req.Server,
		coalesce(req.AccountType, "demo"), now,
	).Scan(
		&a.Id, &a.TenantId, &a.BrokerId, &a.Login, &a.Server,
		&a.AccountType, &a.Currency, &a.Leverage,
		&a.Status, &a.Balance, &a.Equity, &a.Margin, &a.FreeMargin,
		&a.MarginLevel, &a.Profit, &a.ProfitPercent,
		&a.IsDisabled, &a.LastError, &a.Alias,
	)
	if err != nil {
		return nil, fmt.Errorf("insert account: %w", err)
	}
	a.CreatedAt = timestamppb.New(now)
	a.ConnectedAt = nil

	// 3. Test MT connection if broker has mtapi_endpoint
	if brokerHost != "" {
		// Determine platform from broker
		var platform string
		s.pool.QueryRow(ctx, `SELECT platform FROM brokers WHERE id = $1`, req.BrokerId).Scan(&platform)
		gw := s.mt5Gateway
		if strings.EqualFold(platform, "MT4") {
			gw = s.mt4Gateway
		}
		info, err := mtapi.TestConnect(ctx, gw, platform, req.Login, req.Password, brokerHost)
		if err != nil {
			// Connection failed — mark as error but don't delete
			s.pool.Exec(ctx,
				`UPDATE accounts SET status='error', last_error=$1, updated_at=now() WHERE id=$2`,
				err.Error(), a.Id,
			)
			a.Status = "error"
			a.LastError = err.Error()
			s.log.Warn("mt connect failed", zap.String("account", a.Id), zap.Error(err))
		} else {
			// Connection OK — update with account info
			connectedAt := timestamppb.Now()
			s.pool.Exec(ctx, `
				UPDATE accounts SET status='connected', balance=$1, equity=$2, margin=$3,
					free_margin=$4, margin_level=$5, profit=$6, currency=$7, leverage=$8,
					connected_at=now(), updated_at=now()
				WHERE id=$9
			`, info.Balance, info.Equity, info.Margin, info.FreeMargin,
				info.MarginLevel, info.Profit, info.Currency, info.Leverage, a.Id,
			)
			a.Status = "connected"
			a.Balance = info.Balance
			a.Equity = info.Equity
			a.Margin = info.Margin
			a.FreeMargin = info.FreeMargin
			a.MarginLevel = info.MarginLevel
			a.Profit = info.Profit
			if info.Currency != "" {
				a.Currency = info.Currency
			}
			a.Leverage = info.Leverage
			a.ConnectedAt = connectedAt
		}
	} else {
		// No broker endpoint — leave as disconnected (manual setup)
		s.pool.Exec(ctx,
			`UPDATE accounts SET status='disconnected', updated_at=now() WHERE id=$1`, a.Id,
		)
		a.Status = "disconnected"
	}

	return a, nil
}

func (s *Service) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.Account, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	return s.scanAccount(ctx, `SELECT id, tenant_id, broker_id, login, server, account_type, currency, leverage,
		status, balance, equity, margin, free_margin, margin_level, profit, profit_percent,
		is_disabled, last_error, alias, connected_at, created_at
		FROM accounts WHERE id = $1`, req.Id)
}

func (s *Service) ListAccounts(ctx context.Context, req *pb.ListAccountsRequest) (*pb.ListAccountsResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	tenantID := effectiveTenantID(ctx, req.TenantId)
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, broker_id, login, server, account_type, currency, leverage,
			status, balance, equity, margin, free_margin, margin_level, profit, profit_percent,
			is_disabled, last_error, alias, connected_at, created_at
		FROM accounts WHERE tenant_id = $1 ORDER BY login
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*pb.Account
	for rows.Next() {
		a, err := scanAccountRow(rows)
		if err != nil {
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
	return s.scanAccount(ctx, `
		UPDATE accounts SET login=$1, server=$2, account_type=$3, currency=$4, leverage=$5,
			alias=$6, is_disabled=$7, updated_at=now()
		WHERE id=$8
		RETURNING id, tenant_id, broker_id, login, server, account_type, currency, leverage,
			status, balance, equity, margin, free_margin, margin_level, profit, profit_percent,
			is_disabled, last_error, alias, connected_at, created_at
	`, req.Login, req.Server, req.AccountType, req.Currency, req.Leverage,
		req.Alias, req.IsDisabled, req.Id,
	)
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

func (s *Service) ConnectAccount(ctx context.Context, req *pb.ConnectAccountRequest) (*pb.ConnectAccountResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	s.pool.Exec(ctx, `UPDATE accounts SET status='connected', connected_at=now(), updated_at=now() WHERE id=$1`, req.Id)
	return &pb.ConnectAccountResponse{}, nil
}

func (s *Service) DisconnectAccount(ctx context.Context, req *pb.DisconnectAccountRequest) (*pb.DisconnectAccountResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	s.pool.Exec(ctx, `UPDATE accounts SET status='disconnected', updated_at=now() WHERE id=$1`, req.Id)
	return &pb.DisconnectAccountResponse{}, nil
}

// scanAccountRow scans a pgx.Row into an Account proto.
func (s *Service) scanAccount(ctx context.Context, query string, args ...interface{}) (*pb.Account, error) {
	a := &pb.Account{}
	var connectedAt, createdAt interface{}
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&a.Id, &a.TenantId, &a.BrokerId, &a.Login, &a.Server,
		&a.AccountType, &a.Currency, &a.Leverage,
		&a.Status, &a.Balance, &a.Equity, &a.Margin, &a.FreeMargin,
		&a.MarginLevel, &a.Profit, &a.ProfitPercent,
		&a.IsDisabled, &a.LastError, &a.Alias,
		&connectedAt, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	if t, ok := connectedAt.(time.Time); ok && !t.IsZero() {
		a.ConnectedAt = timestamppb.New(t)
	}
	if t, ok := createdAt.(time.Time); ok {
		a.CreatedAt = timestamppb.New(t)
	}
	return a, nil
}

// scanAccountRow scans a pgx.Rows row into an Account proto.
func scanAccountRow(row interface{ Scan(...interface{}) error }) (*pb.Account, error) {
	a := &pb.Account{}
	var connectedAt, createdAt interface{}
	err := row.Scan(
		&a.Id, &a.TenantId, &a.BrokerId, &a.Login, &a.Server,
		&a.AccountType, &a.Currency, &a.Leverage,
		&a.Status, &a.Balance, &a.Equity, &a.Margin, &a.FreeMargin,
		&a.MarginLevel, &a.Profit, &a.ProfitPercent,
		&a.IsDisabled, &a.LastError, &a.Alias,
		&connectedAt, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	if t, ok := connectedAt.(time.Time); ok && !t.IsZero() {
		a.ConnectedAt = timestamppb.New(t)
	}
	if t, ok := createdAt.(time.Time); ok {
		a.CreatedAt = timestamppb.New(t)
	}
	return a, nil
}

func coalesce(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
