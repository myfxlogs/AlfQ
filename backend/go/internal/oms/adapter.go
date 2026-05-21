// Package oms — BrokerAdapter implementations for MT4/MT5 via mtapi gateway.
package oms

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	mt4pb "github.com/alfq/backend/go/gen/mt4"
	mt5pb "github.com/alfq/backend/go/gen/mt5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// BrokerAdapter abstracts broker-specific order operations.
type BrokerAdapter interface {
	Submit(ctx context.Context, req *pb.OrderRequest) (*BrokerResp, error)
	Cancel(ctx context.Context, ticket string) error
	Modify(ctx context.Context, ticket string, price, stopPrice float64) error
	Query(ctx context.Context, ticket string) (*pb.Order, error)
}

// BrokerResp is the broker's response to an order submission.
type BrokerResp struct {
	Ticket    string
	State     pb.OrderState
	FilledQty float64
	FillPrice float64
	ErrorCode int32
	ErrorMsg  string
}

// ── MT5 Adapter ─────────────────────────────────────────────────────

type MT5Adapter struct {
	gatewayAddr string
	login       string
	password    string
	server      string
}

func NewMT5Adapter(gatewayAddr, login, password, server string) *MT5Adapter {
	return &MT5Adapter{gatewayAddr: gatewayAddr, login: login, password: password, server: server}
}

func (a *MT5Adapter) Submit(ctx context.Context, req *pb.OrderRequest) (*BrokerResp, error) {
	conn, err := dialMT(ctx, a.gatewayAddr)
	if err != nil {
		return nil, fmt.Errorf("mt5 adapter: dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	sessionID, err := mt5Connect(ctx, conn, a.login, a.password, a.server)
	if err != nil {
		return nil, fmt.Errorf("mt5 adapter: connect: %w", err)
	}

	ctxSess := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	trading := mt5pb.NewTradingClient(conn)

	op := mt5pb.OrderType_OrderType_Buy
	if req.Side == pb.OrderSide_ORDER_SIDE_SELL {
		op = mt5pb.OrderType_OrderType_Sell
	}

	price := parseMoney(req.GetPrice())
	mtReq := &mt5pb.OrderSendRequest{
		Id:        sessionID,
		Symbol:    req.Symbol,
		Operation: op,
		Volume:    req.Qty,
		Price:     &price,
		Slippage:  ptrUint64(10),
		Comment:   ptrString(req.StrategyId),
	}

	resp, err := trading.OrderSend(ctxSess, mtReq)
	if err != nil {
		return nil, fmt.Errorf("mt5 OrderSend: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return &BrokerResp{
			State:     pb.OrderState_ORDER_STATE_REJECTED,
			ErrorCode: int32(e.GetCode()),
			ErrorMsg:  e.GetMessage(),
		}, nil
	}

	order := resp.GetResult()
	return &BrokerResp{
		Ticket:    fmt.Sprintf("%d", order.GetTicket()),
		State:     pb.OrderState_ORDER_STATE_SUBMITTED,
		FilledQty: float64(order.GetVolume()),
		FillPrice: order.GetOpenPrice(),
	}, nil
}

func (a *MT5Adapter) Cancel(ctx context.Context, ticket string) error { return nil }
func (a *MT5Adapter) Modify(ctx context.Context, ticket string, price, stopPrice float64) error {
	return nil
}
func (a *MT5Adapter) Query(ctx context.Context, ticket string) (*pb.Order, error) { return nil, nil }

// ── MT4 Adapter ─────────────────────────────────────────────────────

type MT4Adapter struct {
	gatewayAddr string
	login       string
	password    string
	server      string
}

func NewMT4Adapter(gatewayAddr, login, password, server string) *MT4Adapter {
	return &MT4Adapter{gatewayAddr: gatewayAddr, login: login, password: password, server: server}
}

func (a *MT4Adapter) Submit(ctx context.Context, req *pb.OrderRequest) (*BrokerResp, error) {
	conn, err := dialMT(ctx, a.gatewayAddr)
	if err != nil {
		return nil, fmt.Errorf("mt4 adapter: dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	sessionID, err := mt4Connect(ctx, conn, a.login, a.password, a.server)
	if err != nil {
		return nil, fmt.Errorf("mt4 adapter: connect: %w", err)
	}

	ctxSess := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	trading := mt4pb.NewTradingClient(conn)

	op := mt4pb.Op_Op_Buy
	if req.Side == pb.OrderSide_ORDER_SIDE_SELL {
		op = mt4pb.Op_Op_Sell
	}

	mtReq := &mt4pb.OrderSendRequest{
		Id:        sessionID,
		Symbol:    req.Symbol,
		Operation: op,
		Volume:    req.Qty,
		Price:     parseMoney(req.GetPrice()),
		Slippage:  10,
		Comment:   req.StrategyId,
	}

	resp, err := trading.OrderSend(ctxSess, mtReq)
	if err != nil {
		return nil, fmt.Errorf("mt4 OrderSend: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return &BrokerResp{
			State:     pb.OrderState_ORDER_STATE_REJECTED,
			ErrorCode: int32(e.GetCode()),
			ErrorMsg:  e.GetMessage(),
		}, nil
	}

	order := resp.GetResult()
	return &BrokerResp{
		Ticket:    fmt.Sprintf("%d", order.GetTicket()),
		State:     pb.OrderState_ORDER_STATE_SUBMITTED,
		FilledQty: order.GetLots(),
		FillPrice: order.GetOpenPrice(),
	}, nil
}

func (a *MT4Adapter) Cancel(ctx context.Context, ticket string) error { return nil }
func (a *MT4Adapter) Modify(ctx context.Context, ticket string, price, stopPrice float64) error {
	return nil
}
func (a *MT4Adapter) Query(ctx context.Context, ticket string) (*pb.Order, error) { return nil, nil }

// ── shared helpers ──────────────────────────────────────────────────

func dialMT(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return grpc.DialContext(dialCtx, addr, //nolint:staticcheck
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
		grpc.WithBlock(), //nolint:staticcheck
	)
}

func mt5Connect(ctx context.Context, conn *grpc.ClientConn, login, password, server string) (string, error) {
	tempID := fmt.Sprintf("oms-%d", time.Now().UnixNano())
	ctxID := metadata.AppendToOutgoingContext(ctx, "id", tempID)
	host, port := splitHostPort(server, "443")
	client := mt5pb.NewConnectionClient(conn)
	resp, err := client.Connect(ctxID, &mt5pb.ConnectRequest{
		User:     parseUint(login),
		Password: password,
		Host:     host,
		Port:     int32(parsePort(port)),
	})
	if err != nil {
		return "", err
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return "", fmt.Errorf("mt5 connect: %s", e.GetMessage())
	}
	return resp.GetResult(), nil
}

func mt4Connect(ctx context.Context, conn *grpc.ClientConn, login, password, server string) (string, error) {
	tempID := fmt.Sprintf("oms-%d", time.Now().UnixNano())
	ctxID := metadata.AppendToOutgoingContext(ctx, "id", tempID)
	host, port := splitHostPort(server, "443")
	client := mt4pb.NewConnectionClient(conn)
	resp, err := client.Connect(ctxID, &mt4pb.ConnectRequest{
		User:     int32(parseUint(login)),
		Password: password,
		Host:     host,
		Port:     int32(parsePort(port)),
	})
	if err != nil {
		return "", err
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		return "", fmt.Errorf("mt4 connect: %s", e.GetMessage())
	}
	return resp.GetResult(), nil
}

func parseMoney(m *pb.Money) float64 {
	if m == nil {
		return 0
	}
	v, _ := strconv.ParseFloat(m.GetValue(), 64)
	return v
}

func splitHostPort(hostPort, defaultPort string) (string, string) {
	for i := len(hostPort) - 1; i >= 0; i-- {
		if hostPort[i] == ':' {
			return hostPort[:i], hostPort[i+1:]
		}
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

func parsePort(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	if n == 0 {
		return 443
	}
	return n
}

func ptrUint64(v uint64) *uint64 { return &v }
func ptrString(v string) *string { return &v }

var _ BrokerAdapter = (*MT4Adapter)(nil)
var _ BrokerAdapter = (*MT5Adapter)(nil)
