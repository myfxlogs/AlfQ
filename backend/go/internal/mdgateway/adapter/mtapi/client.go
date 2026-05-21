// Package mtapi — MT4/MT5 gRPC client: broker search + account connection.
package mtapi

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	mt4pb "github.com/alfq/backend/go/gen/mt4"
	mt5pb "github.com/alfq/backend/go/gen/mt5"
	"github.com/alfq/backend/go/internal/common/config"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AccountInfo returned from MT connection test.
type AccountInfo struct {
	Balance     float64
	Equity      float64
	Margin      float64
	FreeMargin  float64
	MarginLevel float64
	Profit      float64
	Currency    string
	Leverage    int32
}

// PositionInfo is a unified position record from MT4/MT5.
type PositionInfo struct {
	Ticket     int64
	Symbol     string
	Type       string // "buy" | "sell"
	Lots       float64
	OpenPrice  float64
	Profit     float64
	Swap       float64
	Commission float64
}

// HistoryOrderInfo is a unified historical order record from MT4/MT5.
type HistoryOrderInfo struct {
	Ticket     int64
	Symbol     string
	Type       string // "buy" | "sell"
	Lots       float64
	OpenPrice  float64
	ClosePrice float64
	Profit     float64
	Swap       float64
	Commission float64
	OpenTime   string // RFC3339 or empty
	CloseTime  string // RFC3339 or empty
}

// BrokerMatch from online broker search.
type BrokerMatch struct {
	Company string
	Servers []ServerEntry
}

type ServerEntry struct {
	Name   string // e.g. "Exness-Real2"
	Access string // e.g. "63.179.160.248:443"
}

// ── Online broker search ──

// SearchBrokersOnline queries an MT gRPC gateway for broker companies.
func SearchBrokersOnline(ctx context.Context, gw config.GatewayConfig, mtType, company string) ([]BrokerMatch, error) {
	conn, err := dial(ctx, gw)
	if err != nil {
		return nil, fmt.Errorf("mtapi: dial %s gateway: %w", mtType, err)
	}
	defer func() { _ = conn.Close() }()

	switch strings.ToUpper(mtType) {
	case "MT5":
		return searchMT5(ctx, conn, company)
	case "MT4":
		return searchMT4(ctx, conn, company)
	default:
		return nil, fmt.Errorf("mtapi: unsupported platform %q", mtType)
	}
}

func searchMT5(ctx context.Context, conn *grpc.ClientConn, company string) ([]BrokerMatch, error) {
	client := mt5pb.NewServiceClient(conn)
	resp, err := client.Search(ctx, &mt5pb.SearchRequest{Company: company})
	if err != nil {
		return nil, fmt.Errorf("mtapi: mt5 search: %w", err)
	}
	var matches []BrokerMatch
	for _, c := range resp.GetResult() {
		var servers []ServerEntry
		for _, r := range c.GetResults() {
			for _, acc := range r.GetAccess() {
				servers = append(servers, ServerEntry{Name: r.GetName(), Access: acc})
			}
		}
		matches = append(matches, BrokerMatch{Company: c.GetCompanyName(), Servers: servers})
	}
	return matches, nil
}

func searchMT4(ctx context.Context, conn *grpc.ClientConn, company string) ([]BrokerMatch, error) {
	client := mt4pb.NewServiceClient(conn)
	resp, err := client.Search(ctx, &mt4pb.SearchRequest{Company: company})
	if err != nil {
		return nil, fmt.Errorf("mtapi: mt4 search: %w", err)
	}
	var matches []BrokerMatch
	for _, c := range resp.GetResult() {
		var servers []ServerEntry
		for _, r := range c.GetResults() {
			for _, acc := range r.GetAccess() {
				servers = append(servers, ServerEntry{Name: r.GetName(), Access: acc})
			}
		}
		matches = append(matches, BrokerMatch{Company: c.GetCompanyName(), Servers: servers})
	}
	return matches, nil
}

// ── Account connection ──

// ConnectSession dials the gateway, authenticates, and returns a live session ID.
// Caller must call DisconnectSession when done.
func ConnectSession(ctx context.Context, gw config.GatewayConfig, mtType, login, password, brokerHostPort string) (*grpc.ClientConn, string, error) {
	conn, err := dial(ctx, gw)
	if err != nil {
		return nil, "", fmt.Errorf("mtapi: dial gateway: %w", err)
	}

	host, port := splitHostPort(brokerHostPort, "443")

	switch strings.ToUpper(mtType) {
	case "MT5":
		sessionID, err := mt5ConnectSession(ctx, conn, login, password, host, parsePort(port))
		if err != nil {
			_ = conn.Close()
			return nil, "", err
		}
		return conn, sessionID, nil
	case "MT4":
		sessionID, err := mt4ConnectSession(ctx, conn, login, password, host, port)
		if err != nil {
			_ = conn.Close()
			return nil, "", err
		}
		return conn, sessionID, nil
	default:
		_ = conn.Close()
		return nil, "", fmt.Errorf("mtapi: unsupported platform %q", mtType)
	}
}

// DisconnectSession closes a session on the MT gateway.
func DisconnectSession(ctx context.Context, conn *grpc.ClientConn, platform, sessionID string) error {
	if conn == nil {
		return nil
	}
	defer conn.Close()
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	switch strings.ToUpper(platform) {
	case "MT5":
		client := mt5pb.NewConnectionClient(conn)
		_, err := client.Disconnect(ctxWithID, &mt5pb.DisconnectRequest{Id: sessionID})
		return err
	case "MT4":
		client := mt4pb.NewConnectionClient(conn)
		_, err := client.Disconnect(ctxWithID, &mt4pb.DisconnectRequest{Id: sessionID})
		return err
	}
	return nil
}

// TestConnect attempts to connect via gateway and returns account info.
func TestConnect(ctx context.Context, gw config.GatewayConfig, mtType, login, password, brokerHostPort string) (*AccountInfo, error) {
	conn, sessionID, err := ConnectSession(ctx, gw, mtType, login, password, brokerHostPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = DisconnectSession(ctx, conn, mtType, sessionID) }()

	switch strings.ToUpper(mtType) {
	case "MT5":
		return mt5AccountSummary(ctx, conn, sessionID)
	case "MT4":
		return mt4AccountSummary(ctx, conn, sessionID)
	default:
		return nil, fmt.Errorf("mtapi: unsupported platform %q", mtType)
	}
}

func mt5ConnectSession(ctx context.Context, conn *grpc.ClientConn, login, password, host string, port int32) (string, error) {
	tempID := uuid.New().String()
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", tempID)
	connClient := mt5pb.NewConnectionClient(conn)
	resp, err := connClient.Connect(ctxWithID, &mt5pb.ConnectRequest{
		User: parseUint(login), Password: password, Host: host, Port: port,
	})
	if err != nil {
		return "", fmt.Errorf("mtapi: mt5 connect: %w", err)
	}
	if resp.GetError() != nil && resp.GetError().GetMessage() != "" {
		return "", fmt.Errorf("mtapi: mt5 error: %s", resp.GetError().GetMessage())
	}
	return resp.GetResult(), nil
}

func mt5AccountSummary(ctx context.Context, conn *grpc.ClientConn, sessionID string) (*AccountInfo, error) {
	ctxWithSession := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	mt5Client := mt5pb.NewMT5Client(conn)
	summResp, err := mt5Client.AccountSummary(ctxWithSession, &mt5pb.AccountSummaryRequest{Id: sessionID})
	if err != nil {
		return nil, fmt.Errorf("mtapi: mt5 account summary: %w", err)
	}
	summ := summResp.GetResult()
	if summ == nil {
		return &AccountInfo{}, nil
	}
	return &AccountInfo{
		Balance: summ.GetBalance(), Equity: summ.GetEquity(), Margin: summ.GetMargin(),
		FreeMargin: summ.GetFreeMargin(), MarginLevel: summ.GetMarginLevel(),
		Profit: summ.GetProfit(), Currency: summ.GetCurrency(), Leverage: int32(summ.GetLeverage()),
	}, nil
}

func mt4ConnectSession(ctx context.Context, conn *grpc.ClientConn, login, password, host, port string) (string, error) {
	tempID := uuid.New().String()
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", tempID)
	connClient := mt4pb.NewConnectionClient(conn)
	resp, err := connClient.Connect(ctxWithID, &mt4pb.ConnectRequest{
		User: int32(parseUint(login)), Password: password, Host: host, Port: parsePort(port), Id: &tempID,
	})
	if err != nil {
		return "", fmt.Errorf("mtapi: mt4 connect: %w", err)
	}
	if resp.GetError() != nil && resp.GetError().GetMessage() != "" {
		return "", fmt.Errorf("mtapi: mt4 error: %s", resp.GetError().GetMessage())
	}
	return resp.GetResult(), nil
}

func mt4AccountSummary(ctx context.Context, conn *grpc.ClientConn, sessionID string) (*AccountInfo, error) {
	ctxWithToken := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	mt4Client := mt4pb.NewMT4Client(conn)
	summResp, err := mt4Client.AccountSummary(ctxWithToken, &mt4pb.AccountSummaryRequest{Id: sessionID})
	if err != nil {
		return nil, fmt.Errorf("mtapi: mt4 account summary: %w", err)
	}
	summ := summResp.GetResult()
	if summ == nil {
		return &AccountInfo{}, nil
	}
	return &AccountInfo{
		Balance: summ.GetBalance(), Equity: summ.GetEquity(), Margin: summ.GetMargin(),
		FreeMargin: summ.GetFreeMargin(), MarginLevel: summ.GetMarginLevel(),
		Profit: summ.GetProfit(), Currency: summ.GetCurrency(), Leverage: int32(summ.GetLeverage()),
	}, nil
}

// FetchAccountSummary fetches the full account summary using a typed gRPC call on an existing connection.
func FetchAccountSummary(ctx context.Context, conn *grpc.ClientConn, platform, sessionID string) (*AccountInfo, error) {
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	switch strings.ToUpper(platform) {
	case "MT5":
		client := mt5pb.NewMT5Client(conn)
		resp, err := client.AccountSummary(ctxWithID, &mt5pb.AccountSummaryRequest{Id: sessionID})
		if err != nil {
			return nil, fmt.Errorf("mtapi: mt5 account summary: %w", err)
		}
		summ := resp.GetResult()
		if summ == nil {
			return &AccountInfo{}, nil
		}
		return &AccountInfo{
			Balance:     summ.GetBalance(),
			Equity:      summ.GetEquity(),
			Margin:      summ.GetMargin(),
			FreeMargin:  summ.GetFreeMargin(),
			MarginLevel: summ.GetMarginLevel(),
			Profit:      summ.GetProfit(),
			Currency:    summ.GetCurrency(),
			Leverage:    int32(summ.GetLeverage()),
		}, nil
	case "MT4":
		client := mt4pb.NewMT4Client(conn)
		resp, err := client.AccountSummary(ctxWithID, &mt4pb.AccountSummaryRequest{Id: sessionID})
		if err != nil {
			return nil, fmt.Errorf("mtapi: mt4 account summary: %w", err)
		}
		summ := resp.GetResult()
		if summ == nil {
			return &AccountInfo{}, nil
		}
		return &AccountInfo{
			Balance:     summ.GetBalance(),
			Equity:      summ.GetEquity(),
			Margin:      summ.GetMargin(),
			FreeMargin:  summ.GetFreeMargin(),
			MarginLevel: summ.GetMarginLevel(),
			Profit:      summ.GetProfit(),
			Currency:    summ.GetCurrency(),
			Leverage:    int32(summ.GetLeverage()),
		}, nil
	default:
		return nil, fmt.Errorf("mtapi: unsupported platform %q", platform)
	}
}

// FetchOpenedOrders fetches opened orders (positions) from an existing MT connection.
func FetchOpenedOrders(ctx context.Context, conn *grpc.ClientConn, platform, sessionID string) ([]*PositionInfo, error) {
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	switch strings.ToUpper(platform) {
	case "MT5":
		client := mt5pb.NewMT5Client(conn)
		resp, err := client.OpenedOrders(ctxWithID, &mt5pb.OpenedOrdersRequest{Id: sessionID})
		if err != nil {
			return nil, fmt.Errorf("mtapi: mt5 opened orders: %w", err)
		}
		var positions []*PositionInfo
		for _, o := range resp.GetResult() {
			// Only include filled/open positions (not pending orders)
			if o.GetState() != mt5pb.OrderState_OrderState_Filled {
				continue
			}
			posType := "buy"
			if o.GetOrderType() == mt5pb.OrderType_OrderType_Sell {
				posType = "sell"
			}
			positions = append(positions, &PositionInfo{
				Ticket:     o.GetTicket(),
				Symbol:     o.GetSymbol(),
				Type:       posType,
				Lots:       o.GetLots(),
				OpenPrice:  o.GetOpenPrice(),
				Profit:     o.GetProfit(),
				Swap:       o.GetSwap(),
				Commission: o.GetCommission(),
			})
		}
		return positions, nil
	case "MT4":
		client := mt4pb.NewMT4Client(conn)
		resp, err := client.OpenedOrders(ctxWithID, &mt4pb.OpenedOrdersRequest{Id: sessionID})
		if err != nil {
			return nil, fmt.Errorf("mtapi: mt4 opened orders: %w", err)
		}
		var positions []*PositionInfo
		for _, o := range resp.GetResult() {
			// MT4 opened orders are all positions (no pending in OpenedOrders)
			posType := "buy"
			// MT4 Op enum: 0=Buy, 1=Sell, 2=BuyLimit, 3=SellLimit, 4=BuyStop, 5=SellStop
			if o.GetType() == mt4pb.Op_Op_Sell {
				posType = "sell"
			}
			positions = append(positions, &PositionInfo{
				Ticket:     int64(o.GetTicket()),
				Symbol:     o.GetSymbol(),
				Type:       posType,
				Lots:       o.GetLots(),
				OpenPrice:  o.GetOpenPrice(),
				Profit:     o.GetProfit(),
				Swap:       o.GetSwap(),
				Commission: o.GetCommission(),
			})
		}
		return positions, nil
	default:
		return nil, fmt.Errorf("mtapi: unsupported platform %q", platform)
	}
}

// FetchOrderHistory fetches closed order history from an existing MT connection.
func FetchOrderHistory(ctx context.Context, conn *grpc.ClientConn, platform, sessionID, from, to string) ([]*HistoryOrderInfo, error) {
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	switch strings.ToUpper(platform) {
	case "MT5":
		client := mt5pb.NewMT5Client(conn)
		req := &mt5pb.OrderHistoryRequest{Id: sessionID, From: from, To: to}
		resp, err := client.OrderHistory(ctxWithID, req)
		if err != nil {
			return nil, fmt.Errorf("mtapi: mt5 order history: %w", err)
		}
		if e := resp.GetError(); e != nil && e.GetMessage() != "" {
			return nil, fmt.Errorf("mtapi: mt5 order history error: %s", e.GetMessage())
		}
		orders := make([]*HistoryOrderInfo, 0, len(resp.GetResult()))
		for _, o := range resp.GetResult() {
			// Skip non-trade orders (deposit/withdraw etc are OrderType >= 100).
			if o.GetOrderType() >= mt5pb.OrderType_OrderType_Balance {
				continue
			}
			orderType := "buy"
			if o.GetOrderType() == mt5pb.OrderType_OrderType_Sell {
				orderType = "sell"
			}
			orders = append(orders, &HistoryOrderInfo{
				Ticket:     o.GetTicket(),
				Symbol:     o.GetSymbol(),
				Type:       orderType,
				Lots:       o.GetLots(),
				OpenPrice:  o.GetOpenPrice(),
				ClosePrice: o.GetClosePrice(),
				Profit:     o.GetProfit(),
				Swap:       o.GetSwap(),
				Commission: o.GetCommission(),
				OpenTime:   timestampToRFC3339(o.GetOpenTime()),
				CloseTime:  timestampToRFC3339(o.GetCloseTime()),
			})
		}
		return orders, nil
	case "MT4":
		client := mt4pb.NewMT4Client(conn)
		req := &mt4pb.OrderHistoryRequest{Id: sessionID, From: from, To: to}
		resp, err := client.OrderHistory(ctxWithID, req)
		if err != nil {
			return nil, fmt.Errorf("mtapi: mt4 order history: %w", err)
		}
		if e := resp.GetError(); e != nil && e.GetMessage() != "" {
			return nil, fmt.Errorf("mtapi: mt4 order history error: %s", e.GetMessage())
		}
		orders := make([]*HistoryOrderInfo, 0, len(resp.GetResult()))
		for _, o := range resp.GetResult() {
			// MT4 Op enum: 6=Balance, 7=Credit — skip non-trade entries.
			t := o.GetType()
			if int32(t) >= 6 {
				continue
			}
			orderType := "buy"
			if t == mt4pb.Op_Op_Sell {
				orderType = "sell"
			}
			orders = append(orders, &HistoryOrderInfo{
				Ticket:     int64(o.GetTicket()),
				Symbol:     o.GetSymbol(),
				Type:       orderType,
				Lots:       o.GetLots(),
				OpenPrice:  o.GetOpenPrice(),
				ClosePrice: o.GetClosePrice(),
				Profit:     o.GetProfit(),
				Swap:       o.GetSwap(),
				Commission: o.GetCommission(),
				OpenTime:   timestampToRFC3339(o.GetOpenTime()),
				CloseTime:  timestampToRFC3339(o.GetCloseTime()),
			})
		}
		return orders, nil
	default:
		return nil, fmt.Errorf("mtapi: unsupported platform %q", platform)
	}
}

// DialAndFetchOrderHistory dials the mtapi gateway, connects to the broker,
// fetches order history for the given time window, and disconnects.
//
// Deprecated: use mthub.Client.OrderHistory instead. This function is retained
// for CLI fallback (symbol-sync, md-backfill) until MH-4 wires those tools.
func DialAndFetchOrderHistory(ctx context.Context, gatewayAddr, platform, login, password, server, from, to string) ([]*HistoryOrderInfo, error) {
	conn, err := dialAddr(ctx, gatewayAddr)
	if err != nil {
		return nil, fmt.Errorf("mtapi: dial: %w", err)
	}
	defer conn.Close()

	host, port := splitHostPort(server, "443")
	sessionID, err := connectSession(ctx, conn, platform, login, password, host, port)
	if err != nil {
		return nil, fmt.Errorf("mtapi: connect: %w", err)
	}

	return FetchOrderHistory(ctx, conn, platform, sessionID, from, to)
}

// dialAddr dials the mtapi gateway directly (TLS).
func dialAddr(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return grpc.DialContext(dialCtx, addr, //nolint:staticcheck
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
		grpc.WithBlock(), //nolint:staticcheck
	)
}

// connectSession creates a broker session via the given connection.
func connectSession(ctx context.Context, conn *grpc.ClientConn, platform, login, password, host, port string) (string, error) {
	switch strings.ToUpper(platform) {
	case "MT5":
		return mt5ConnectSession(ctx, conn, login, password, host, parsePort(port))
	case "MT4":
		return mt4ConnectSession(ctx, conn, login, password, host, port)
	default:
		return "", fmt.Errorf("mtapi: unknown platform %q", platform)
	}
}

func timestampToRFC3339(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return ""
	}
	return ts.AsTime().UTC().Format(time.RFC3339)
}

// // getAccountSummary is a legacy helper for reflection-based RPC calls.
// // Deprecated: use typed FetchAccountSummary instead.
// func getAccountSummary(ctx context.Context, conn *grpc.ClientConn, method string) (*AccountInfo, error) {
// 	summary := make(map[string]interface{})
// 	_ = conn.Invoke(ctx, method, map[string]interface{}{}, summary)
// 	result, _ := summary["result"].(map[string]interface{})
// 	if result == nil {
// 		return &AccountInfo{}, nil
// 	}
// 	return &AccountInfo{
// 		Balance:     getFloat(result, "balance"),
// 		Equity:      getFloat(result, "equity"),
// 		Margin:      getFloat(result, "margin"),
// 		FreeMargin:  getFloat(result, "freeMargin"),
// 		MarginLevel: getFloat(result, "marginLevel"),
// 		Profit:      getFloat(result, "profit"),
// 		Currency:    getString(result, "currency"),
// 		Leverage:    int32(getFloat(result, "leverage")),
// 	}, nil
// }

// ── internal ──

func dial(ctx context.Context, gw config.GatewayConfig) (*grpc.ClientConn, error) {
	dialOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTimeout(gw.Timeout)} //nolint:staticcheck
	if gw.UseTLS {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	return grpc.DialContext(ctx, gw.Addr, dialOpts...) //nolint:staticcheck
}

func splitHostPort(hostPort, defaultPort string) (string, string) {
	parts := strings.Split(hostPort, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return hostPort, defaultPort
}

func parseUint(s string) uint64 {
	var n uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint64(c-'0')
		}
	}
	return n
}

func parsePort(s string) int32 {
	n := parseUint(s)
	if n == 0 {
		return 443
	}
	return int32(n)
}

// func getFloat(m map[string]interface{}, key string) float64 {
// 	if v, ok := m[key].(float64); ok { return v }
// 	return 0
// }
// func getString(m map[string]interface{}, key string) string {
// 	s, _ := m[key].(string)
// 	return s
// }
