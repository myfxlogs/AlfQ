package mthub

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	mthubv1 "github.com/alfq/backend/go/gen/alfq/mthub/v1"
	"github.com/alfq/backend/go/gen/alfq/mthub/v1/mthubv1connect"
)

// Client is a convenience wrapper around the generated ConnectRPC MtHubService client.
type Client struct {
	rpc mthubv1connect.MtHubServiceClient
}

// NewClient creates a mthub Client connected to md-gateway's internal HTTP port.
func NewClient(mdGatewayAddr string) *Client {
	baseURL := fmt.Sprintf("http://%s", mdGatewayAddr)
	return &Client{
		rpc: mthubv1connect.NewMtHubServiceClient(http.DefaultClient, baseURL),
	}
}

// ── Session ──

// EnsureSessionResult holds the result of an EnsureSession call.
type EnsureSessionResult struct {
	SessionID     string
	AlreadyActive bool
}

// EnsureSession ensures a session exists for the given account.
func (c *Client) EnsureSession(ctx context.Context, accountID string) (*EnsureSessionResult, error) {
	resp, err := c.rpc.EnsureSession(ctx, connect.NewRequest(&mthubv1.EnsureSessionRequest{
		AccountId: accountID,
	}))
	if err != nil {
		return nil, err
	}
	return &EnsureSessionResult{
		SessionID:     resp.Msg.SessionId,
		AlreadyActive: resp.Msg.AlreadyActive,
	}, nil
}

// CloseSession closes a session for the given account.
func (c *Client) CloseSession(ctx context.Context, accountID string) error {
	_, err := c.rpc.CloseSession(ctx, connect.NewRequest(&mthubv1.CloseSessionRequest{
		AccountId: accountID,
	}))
	return err
}

// ── Order History ──

// OrderRecord is a flat order struct used by callers.
type OrderRecord struct {
	Ticket       int64
	Symbol       string
	Side         string
	Lots         float64
	OpenPrice    float64
	ClosePrice   float64
	Profit       float64
	Swap         float64
	Commission   float64
	OpenTime     string
	CloseTime    string
	State        string
	OpenTimeMs   int64
	CurrentPrice float64
}

// OrderHistory fetches closed orders in the given time window.
func (c *Client) OrderHistory(ctx context.Context, accountID, from, to string) ([]*OrderRecord, error) {
	resp, err := c.rpc.OrderHistory(ctx, connect.NewRequest(&mthubv1.OrderHistoryRequest{
		AccountId: accountID,
		From:      from,
		To:        to,
	}))
	if err != nil {
		return nil, err
	}
	out := make([]*OrderRecord, 0, len(resp.Msg.Orders))
	for _, o := range resp.Msg.Orders {
		out = append(out, &OrderRecord{
			Ticket: o.Ticket, Symbol: o.Symbol, Side: o.Side, Lots: o.Lots,
			OpenPrice: o.OpenPrice, ClosePrice: o.ClosePrice,
			Profit: o.Profit, Swap: o.Swap, Commission: o.Commission,
			OpenTime: o.OpenTime, CloseTime: o.CloseTime, State: o.State,
		})
	}
	return out, nil
}

// OpenedOrders fetches currently open positions.
func (c *Client) OpenedOrders(ctx context.Context, accountID string) ([]*OrderRecord, error) {
	resp, err := c.rpc.OpenedOrders(ctx, connect.NewRequest(&mthubv1.OpenedOrdersRequest{
		AccountId: accountID,
	}))
	if err != nil {
		return nil, err
	}
	out := make([]*OrderRecord, 0, len(resp.Msg.Orders))
	for _, o := range resp.Msg.Orders {
		out = append(out, &OrderRecord{
			Ticket: o.Ticket, Symbol: o.Symbol, Side: o.Side, Lots: o.Lots,
			OpenPrice: o.OpenPrice, Profit: o.Profit, Swap: o.Swap, Commission: o.Commission,
			OpenTimeMs: o.OpenTimeMs, CurrentPrice: o.CurrentPrice,
		})
	}
	return out, nil
}

// ── Stream ──

// OrderEvent is a flat order event from the stream.
type OrderEvent struct {
	AccountID string
	Type      string
	Order     *OrderRecord
}

// SubscribeOrderEvents opens a server-stream for order events.
// Returns a channel that receives OrderEvents. Call cancel to stop.
func (c *Client) SubscribeOrderEvents(ctx context.Context, accountID string) (<-chan *OrderEvent, context.CancelFunc, error) {
	stream, err := c.rpc.SubscribeOrderEvents(ctx, connect.NewRequest(&mthubv1.SubscribeOrderEventsRequest{
		AccountId: accountID,
	}))
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan *OrderEvent, 64)
	streamCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(ch)
		for stream.Receive() {
			ev := stream.Msg()
			oe := &OrderEvent{
				AccountID: ev.AccountId,
				Type:      ev.Type,
			}
			if ev.Order != nil {
				oe.Order = &OrderRecord{
					Ticket: ev.Order.Ticket, Symbol: ev.Order.Symbol,
					Side: ev.Order.Side, Lots: ev.Order.Lots,
					OpenPrice: ev.Order.OpenPrice, ClosePrice: ev.Order.ClosePrice,
					Profit: ev.Order.Profit, Swap: ev.Order.Swap, Commission: ev.Order.Commission,
					OpenTime: ev.Order.OpenTime, CloseTime: ev.Order.CloseTime, State: ev.Order.State,
				}
			}
			select {
			case ch <- oe:
			case <-streamCtx.Done():
				return
			}
		}
	}()

	return ch, cancel, nil
}
