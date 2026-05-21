package mthub

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	mthubv1 "github.com/alfq/backend/go/gen/alfq/mthub/v1"
	"github.com/alfq/backend/go/internal/mdgateway/adapter/mtapi"
	"go.uber.org/zap"
)

// MtHubService implements alfq.mthub.v1.MtHubServiceHandler (ConnectRPC).
type MtHubService struct {
	hub    *Hub
	events *OrderEventBroker
	log    *zap.Logger
}

func NewMtHubService(hub *Hub, events *OrderEventBroker, log *zap.Logger) *MtHubService {
	return &MtHubService{hub: hub, events: events, log: log}
}

// ── Session management ──

func (s *MtHubService) EnsureSession(
	ctx context.Context,
	req *connect.Request[mthubv1.EnsureSessionRequest],
) (*connect.Response[mthubv1.EnsureSessionResponse], error) {
	ses, err := s.hub.EnsureSession(req.Msg.AccountId, req.Msg.AccountId)
	if err != nil {
		return nil, err
	}
	if ses == nil {
		return nil, fmt.Errorf("mthub: session not found for account %s", req.Msg.AccountId)
	}
	return connect.NewResponse(&mthubv1.EnsureSessionResponse{
		SessionId:     ses.SessionID(),
		AlreadyActive: ses.SessionID() != "",
	}), nil
}

func (s *MtHubService) CloseSession(
	ctx context.Context,
	req *connect.Request[mthubv1.CloseSessionRequest],
) (*connect.Response[mthubv1.CloseSessionResponse], error) {
	s.events.Unsubscribe(req.Msg.AccountId)
	s.hub.CloseSession(req.Msg.AccountId)
	return connect.NewResponse(&mthubv1.CloseSessionResponse{}), nil
}

// ── OMS (stub — wired in MH-3) ──

func (s *MtHubService) OrderSend(
	ctx context.Context,
	req *connect.Request[mthubv1.OrderSendRequest],
) (*connect.Response[mthubv1.OrderSendResponse], error) {
	return connect.NewResponse(&mthubv1.OrderSendResponse{
		Error: "mthub: OrderSend not yet implemented (MH-3)",
	}), nil
}

func (s *MtHubService) OrderClose(
	ctx context.Context,
	req *connect.Request[mthubv1.OrderCloseRequest],
) (*connect.Response[mthubv1.OrderCloseResponse], error) {
	return connect.NewResponse(&mthubv1.OrderCloseResponse{
		Error: "mthub: OrderClose not yet implemented (MH-3)",
	}), nil
}

// ── History ──

func (s *MtHubService) OrderHistory(
	ctx context.Context,
	req *connect.Request[mthubv1.OrderHistoryRequest],
) (*connect.Response[mthubv1.OrderHistoryResponse], error) {
	ses, err := s.hub.EnsureSession(req.Msg.AccountId, req.Msg.AccountId)
	if err != nil || ses == nil {
		return nil, fmt.Errorf("mthub: session not found: %w", err)
	}
	conn := ses.Conn()
	if conn == nil {
		return nil, fmt.Errorf("mthub: not connected")
	}
	orders, err := mtapi.FetchOrderHistory(ctx, conn, ses.Platform(), ses.SessionID(), req.Msg.From, req.Msg.To)
	if err != nil {
		return nil, err
	}
	out := make([]*mthubv1.OrderRecord, 0, len(orders))
	for _, o := range orders {
		out = append(out, toOrderRecord(o))
	}
	return connect.NewResponse(&mthubv1.OrderHistoryResponse{Orders: out}), nil
}

func (s *MtHubService) OpenedOrders(
	ctx context.Context,
	req *connect.Request[mthubv1.OpenedOrdersRequest],
) (*connect.Response[mthubv1.OpenedOrdersResponse], error) {
	ses, err := s.hub.EnsureSession(req.Msg.AccountId, req.Msg.AccountId)
	if err != nil || ses == nil {
		return nil, fmt.Errorf("mthub: session not found: %w", err)
	}
	conn := ses.Conn()
	if conn == nil {
		return nil, fmt.Errorf("mthub: not connected")
	}
	positions, err := mtapi.FetchOpenedOrders(ctx, conn, ses.Platform(), ses.SessionID())
	if err != nil {
		return nil, err
	}
	out := make([]*mthubv1.OrderRecord, 0, len(positions))
	for _, p := range positions {
		out = append(out, &mthubv1.OrderRecord{
			Ticket: p.Ticket, Symbol: p.Symbol, Side: p.Type, Lots: p.Lots,
			OpenPrice: p.OpenPrice, Profit: p.Profit, Swap: p.Swap, Commission: p.Commission,
		})
	}
	return connect.NewResponse(&mthubv1.OpenedOrdersResponse{Orders: out}), nil
}

// ── Symbol + price (stub — wired in MH-4) ──

func (s *MtHubService) SymbolParamsMany(
	ctx context.Context,
	req *connect.Request[mthubv1.SymbolParamsManyRequest],
) (*connect.Response[mthubv1.SymbolParamsManyResponse], error) {
	return connect.NewResponse(&mthubv1.SymbolParamsManyResponse{}), nil
}

func (s *MtHubService) PriceHistory(
	ctx context.Context,
	req *connect.Request[mthubv1.PriceHistoryRequest],
) (*connect.Response[mthubv1.PriceHistoryResponse], error) {
	return connect.NewResponse(&mthubv1.PriceHistoryResponse{}), nil
}

// ── Stream ──

func (s *MtHubService) SubscribeOrderEvents(
	ctx context.Context,
	req *connect.Request[mthubv1.SubscribeOrderEventsRequest],
	stream *connect.ServerStream[mthubv1.OrderEvent],
) error {
	ch := s.events.Subscribe(req.Msg.AccountId)
	defer s.events.Unsubscribe(req.Msg.AccountId)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(ev); err != nil {
				return err
			}
		}
	}
}

// ── helpers ──

func toOrderRecord(o *mtapi.HistoryOrderInfo) *mthubv1.OrderRecord {
	return &mthubv1.OrderRecord{
		Ticket: o.Ticket, Symbol: o.Symbol, Side: o.Type, Lots: o.Lots,
		OpenPrice: o.OpenPrice, ClosePrice: o.ClosePrice,
		Profit: o.Profit, Swap: o.Swap, Commission: o.Commission,
		OpenTime: o.OpenTime, CloseTime: o.CloseTime,
	}
}
