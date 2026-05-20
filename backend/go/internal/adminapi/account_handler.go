// Package adminapi — AccountService RPC handler implementations.
package adminapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/mdgateway/adapter/mtapi"
	"github.com/jackc/pgx/v5/pgconn"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"go.uber.org/zap"
)

// directBrokerID is the placeholder broker used for direct server binding
// (when no broker record matches the user's selection).
// Must exist in the brokers table (seed: 00000000-0000-0000-0000-000000000000).
const directBrokerID = "00000000-0000-0000-0000-000000000000"

func (s *Service) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.Account, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}

	// 0. Validate inputs — backend must never trust frontend data
	if strings.TrimSpace(req.Login) == "" {
		return nil, fmt.Errorf("交易账号不能为空")
	}
	if strings.TrimSpace(req.Password) == "" {
		return nil, fmt.Errorf("密码不能为空")
	}
	if req.BrokerId == "" {
		if strings.TrimSpace(req.Server) == "" {
			return nil, fmt.Errorf("服务器地址不能为空")
		}
		if mt := strings.ToUpper(strings.TrimSpace(req.MtType)); mt != "MT4" && mt != "MT5" {
			return nil, fmt.Errorf("交易平台类型无效：%s（仅支持 MT4/MT5）", req.MtType)
		}
	}

	// 1. Look up broker to get MT endpoint (skip for placeholder ID)
	var brokerHost, platform string
	if req.BrokerId == "" {
		brokerHost = req.Server // use server field as host:port directly
		platform = req.MtType
	} else {
		if err := s.pool.QueryRow(ctx,
			`SELECT COALESCE(mtapi_endpoint, ''), platform FROM brokers WHERE id = $1`,
			req.BrokerId,
		).Scan(&brokerHost, &platform); err != nil {
			return nil, fmt.Errorf("broker lookup: %w", err)
		}
	}

	// 2. Insert account with connecting status
	tid := effectiveTenantID(ctx, req.TenantId)
	uid := auth.UserFromContext(ctx)
	if uid == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	a := &pb.Account{}
	now := time.Now()

	// broker_id: use direct placeholder when no broker lookup
	bid := req.BrokerId
	if bid == "" {
		bid = directBrokerID
	}

	err := s.pool.QueryRow(ctx, `
		INSERT INTO accounts (tenant_id, user_id, broker_id, login, password, server, server_name, platform, account_type,
			status, currency, leverage, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'connecting', 'USD', 100, $10, $10)
		RETURNING id, tenant_id, user_id, broker_id, login, server, server_name, account_type, currency, leverage,
			status, balance, equity, margin, free_margin, margin_level, profit, profit_percent,
			is_disabled, last_error, alias, platform
	`, tid, uid, bid, req.Login, req.Password, req.Server, req.ServerName,
		strings.ToLower(req.MtType), coalesce(req.AccountType, "demo"), now,
	).Scan(
		&a.Id, &a.TenantId, new(string), // user_id skipped (not in proto)
		&a.BrokerId, &a.Login, &a.Server, &a.ServerName,
		&a.AccountType, &a.Currency, &a.Leverage,
		&a.Status, &a.Balance, &a.Equity, &a.Margin, &a.FreeMargin,
		&a.MarginLevel, &a.Profit, &a.ProfitPercent,
		&a.IsDisabled, &a.LastError, &a.Alias, &a.Platform,
	)
	a.Platform = strings.ToUpper(a.Platform)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("该交易账号已被绑定，请勿重复绑定")
		}
		return nil, fmt.Errorf("insert account: %w", err)
	}
	a.CreatedAt = timestamppb.New(now)
	a.ConnectedAt = nil

	// 3. Test MT connection if broker has host
	if brokerHost != "" {
		gw := s.mt5Gateway
		if strings.EqualFold(platform, "MT4") {
			gw = s.mt4Gateway
		}

		// 创建带超时的 context (默认 30s，对齐 GatewayConfig.Timeout)
		timeout := gw.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		connectCtx, cancel := context.WithTimeout(ctx, timeout)
		info, err := mtapi.TestConnect(connectCtx, gw, platform, req.Login, req.Password, brokerHost)
		cancel()

		if err != nil {
			// 验证失败 → 彻底删除数据库记录 + 返回友好错误
			s.pool.Exec(context.Background(),
				`DELETE FROM accounts WHERE id = $1`, a.Id,
			)
			// 日志中不记录密码
			s.log.Warn("mt connect failed",
				zap.String("account_id", a.Id),
				zap.String("login", req.Login),
				zap.String("platform", platform),
				zap.Error(err),
			)
			return nil, translateMTError(err, platform)
		}

		// 连接成功 → 更新账户信息
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

		// Start persistent long-lived connection (event-driven streaming + auto-reconnect)
		if s.acctConn != nil {
			s.acctConn.Connect(context.Background(), AccountInfo{
				ID: a.Id, Login: req.Login, Password: req.Password,
				Server: brokerHost, Platform: platform,
			})
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

// translateMTError converts raw MT connection errors into user-friendly messages.
func translateMTError(err error, platform string) error {
	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout"):
		return fmt.Errorf("连接交易服务器超时（30秒），请检查网络或服务器地址后重试")
	case strings.Contains(lower, "authorization failed") || strings.Contains(lower, "invalid account"):
		return fmt.Errorf("账号或只读密码错误，请核实后重试。注意：请使用%s平台的「观摩密码」(Investor Password)，而非交易密码", platform)
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "unreachable"):
		return fmt.Errorf("无法连接到%s交易服务器 %s，请检查服务器地址是否正确", platform, parseHostFromError(msg))
	case strings.Contains(lower, "unsupported"):
		return fmt.Errorf("不支持的交易平台类型：%s", platform)
	case strings.Contains(lower, "invalidargument") || strings.Contains(lower, "id header"):
		return fmt.Errorf("%s网关连接参数错误，请检查服务器地址配置", platform)
	case strings.Contains(lower, "credential") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "unauthenticated"):
		return fmt.Errorf("%s认证失败，请检查账号和只读密码", platform)
	case strings.Contains(lower, "disconnected") || strings.Contains(lower, "disposed object") || strings.Contains(lower, "cannot access"):
		return fmt.Errorf("无法连接到%s交易服务器，请检查服务器地址、账号和只读密码是否正确", platform)
	case strings.Contains(lower, "send login"):
		return fmt.Errorf("%s验证失败：账号或只读密码错误，请核实后重试", platform)
	default:
		return fmt.Errorf("%s验证失败：%s", platform, msg)
	}
}

// parseHostFromError extracts a host string from an error for display purposes.
func parseHostFromError(msg string) string {
	if idx := strings.Index(msg, "dial tcp "); idx >= 0 {
		rest := msg[idx+len("dial tcp "):]
		if end := strings.IndexAny(rest, ": "); end > 0 {
			return rest[:end]
		}
	}
	return ""
}

func (s *Service) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.Account, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	return s.scanAccount(ctx, `SELECT id, tenant_id, user_id, broker_id, login, server, server_name, account_type, currency, leverage,
		status, balance, equity, margin, free_margin, margin_level, profit, profit_percent,
		is_disabled, last_error, alias, platform, connected_at, created_at
		FROM accounts WHERE id = $1`, req.Id)
}

func (s *Service) ListAccounts(ctx context.Context, req *pb.ListAccountsRequest) (*pb.ListAccountsResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	uid := auth.UserFromContext(ctx)
	if uid == "" {
		return nil, fmt.Errorf("user not authenticated")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, user_id, broker_id, login, server, server_name, account_type, currency, leverage,
			status, balance, equity, margin, free_margin, margin_level, profit, profit_percent,
			is_disabled, last_error, alias, platform, connected_at, created_at
		FROM accounts
		WHERE user_id = $1
		ORDER BY login
	`, uid)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*pb.Account
	summary := &pb.AccountSummary{}
	for rows.Next() {
		a, err := scanAccountRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, a)
		if a.Status == "connected" {
			summary.ConnectedCount++
		}
		summary.TotalBalance += a.Balance
		summary.TotalEquity += a.Equity
		summary.TotalProfit += a.Profit
	}
	return &pb.ListAccountsResponse{Accounts: accounts, Summary: summary}, rows.Err()
}

func (s *Service) UpdateAccount(ctx context.Context, req *pb.Account) (*pb.Account, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	return s.scanAccount(ctx, `
		UPDATE accounts SET login=$1, server=$2, account_type=$3, currency=$4, leverage=$5,
			alias=$6, is_disabled=$7, updated_at=now()
		WHERE id=$8
		RETURNING id, tenant_id, user_id, broker_id, login, server, server_name, account_type, currency, leverage,
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

// scanAccount scans a single account row (including user_id placeholder) into an Account proto.
func (s *Service) scanAccount(ctx context.Context, query string, args ...interface{}) (*pb.Account, error) {
	a := &pb.Account{}
	var connectedAt, createdAt interface{}
	var platform string
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&a.Id, &a.TenantId, new(string), // user_id skipped (not in proto)
		&a.BrokerId, &a.Login, &a.Server, &a.ServerName,
		&a.AccountType, &a.Currency, &a.Leverage,
		&a.Status, &a.Balance, &a.Equity, &a.Margin, &a.FreeMargin,
		&a.MarginLevel, &a.Profit, &a.ProfitPercent,
		&a.IsDisabled, &a.LastError, &a.Alias, &platform,
		&connectedAt, &createdAt,
	)
	a.Platform = strings.ToUpper(platform)
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

// scanAccountRow scans a pgx.Rows row (including user_id placeholder) into an Account proto.
func scanAccountRow(row interface{ Scan(...interface{}) error }) (*pb.Account, error) {
	a := &pb.Account{}
	var connectedAt, createdAt interface{}
	var platform string
	err := row.Scan(
		&a.Id, &a.TenantId, new(string), // user_id skipped (not in proto)
		&a.BrokerId, &a.Login, &a.Server, &a.ServerName,
		&a.AccountType, &a.Currency, &a.Leverage,
		&a.Status, &a.Balance, &a.Equity, &a.Margin, &a.FreeMargin,
		&a.MarginLevel, &a.Profit, &a.ProfitPercent,
		&a.IsDisabled, &a.LastError, &a.Alias, &platform,
		&connectedAt, &createdAt,
	)
	a.Platform = strings.ToUpper(platform)
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
