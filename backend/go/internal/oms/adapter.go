// Package oms — BrokerAdapter interface and MT4/MT5 adapter skeleton.
//
// MT4 and MT5 are completely separate platforms per docs/14 §3.4.
// Each has its own proto types and adapter implementation.
// Only the BrokerAdapter interface and domain types are shared.
package oms

import (
	"context"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
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
	Ticket     string
	State      pb.OrderState
	FilledQty  float64
	FillPrice  float64
	ErrorCode  int32
	ErrorMsg   string
}

// MT4Adapter implements BrokerAdapter for MetaTrader 4.
type MT4Adapter struct {
	// In production: *mt4.Client from internal/mdgateway/adapter/mt4
	login    string
	password string
	server   string
}

func NewMT4Adapter(login, password, server string) *MT4Adapter {
	return &MT4Adapter{login: login, password: password, server: server}
}

func (a *MT4Adapter) Submit(ctx context.Context, req *pb.OrderRequest) (*BrokerResp, error) {
	// TODO: mt4.Client.Trading.OrderSend(ctx, req)
	return &BrokerResp{State: pb.OrderState_ORDER_STATE_SUBMITTED}, nil
}

func (a *MT4Adapter) Cancel(ctx context.Context, ticket string) error { return nil }
func (a *MT4Adapter) Modify(ctx context.Context, ticket string, price, stopPrice float64) error {
	return nil
}
func (a *MT4Adapter) Query(ctx context.Context, ticket string) (*pb.Order, error) {
	return nil, nil
}

// MT5Adapter implements BrokerAdapter for MetaTrader 5.
type MT5Adapter struct {
	login    string
	password string
	server   string
}

func NewMT5Adapter(login, password, server string) *MT5Adapter {
	return &MT5Adapter{login: login, password: password, server: server}
}

func (a *MT5Adapter) Submit(ctx context.Context, req *pb.OrderRequest) (*BrokerResp, error) {
	// TODO: mt5.Client.Trading.OrderSend(ctx, req)
	return &BrokerResp{State: pb.OrderState_ORDER_STATE_SUBMITTED}, nil
}

func (a *MT5Adapter) Cancel(ctx context.Context, ticket string) error { return nil }
func (a *MT5Adapter) Modify(ctx context.Context, ticket string, price, stopPrice float64) error {
	return nil
}
func (a *MT5Adapter) Query(ctx context.Context, ticket string) (*pb.Order, error) {
	return nil, nil
}

// Ensure interface compliance.
var _ BrokerAdapter = (*MT4Adapter)(nil)
var _ BrokerAdapter = (*MT5Adapter)(nil)
