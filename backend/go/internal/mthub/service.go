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
	ses, err := s.hub.EnsureSession(req.Msg.AccountId, "")
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
	ses, err := s.hub.EnsureSession(req.Msg.AccountId, "")
	if err != nil {
		return nil, fmt.Errorf("mthub: ensure session: %w", err)
	}
	if ses == nil {
		return nil, fmt.Errorf("mthub: no session for account %s", req.Msg.AccountId)
	}
	conn := ses.Conn()
	if conn == nil {
		return nil, fmt.Errorf("mthub: account %s not connected", req.Msg.AccountId)
	}

	ticket, err := mtapi.PlaceOrder(ctx, conn, ses.Platform(), ses.SessionID(), &mtapi.OrderRequest{
		Symbol:     req.Msg.Symbol,
		Side:       req.Msg.Side,
		Volume:     req.Msg.Lots,
		Price:      req.Msg.Price,
		Slippage:   3,
		StopLoss:   req.Msg.Sl,
		TakeProfit: req.Msg.Tp,
		Comment:    req.Msg.Comment,
	})
	if err != nil {
		s.log.Warn("ordersend failed",
			zap.String("account_id", req.Msg.AccountId),
			zap.String("symbol", req.Msg.Symbol),
			zap.Error(err),
		)
		return connect.NewResponse(&mthubv1.OrderSendResponse{
			Error: err.Error(),
		}), nil
	}

	s.log.Info("order placed",
		zap.String("account_id", req.Msg.AccountId),
		zap.String("symbol", req.Msg.Symbol),
		zap.Int64("ticket", ticket),
	)
	return connect.NewResponse(&mthubv1.OrderSendResponse{
		Ticket: ticket,
	}), nil
}

func (s *MtHubService) OrderClose(
	ctx context.Context,
	req *connect.Request[mthubv1.OrderCloseRequest],
) (*connect.Response[mthubv1.OrderCloseResponse], error) {
	ses, err := s.hub.EnsureSession(req.Msg.AccountId, "")
	if err != nil {
		return nil, fmt.Errorf("mthub: ensure session: %w", err)
	}
	if ses == nil {
		return nil, fmt.Errorf("mthub: no session for account %s", req.Msg.AccountId)
	}
	conn := ses.Conn()
	if conn == nil {
		return nil, fmt.Errorf("mthub: not connected")
	}
	if err := mtapi.CloseOrder(ctx, conn, ses.Platform(), ses.SessionID(), req.Msg.Ticket, req.Msg.Lots); err != nil {
		s.log.Warn("orderclose failed", zap.Int64("ticket", req.Msg.Ticket), zap.Error(err))
		return connect.NewResponse(&mthubv1.OrderCloseResponse{Error: err.Error()}), nil
	}
	s.log.Info("order closed", zap.Int64("ticket", req.Msg.Ticket))
	return connect.NewResponse(&mthubv1.OrderCloseResponse{}), nil
}

// ── History ──

func (s *MtHubService) OrderHistory(
	ctx context.Context,
	req *connect.Request[mthubv1.OrderHistoryRequest],
) (*connect.Response[mthubv1.OrderHistoryResponse], error) {
	ses, err := s.hub.EnsureSession(req.Msg.AccountId, "")
	if err != nil {
		return nil, fmt.Errorf("mthub: ensure session: %w", err)
	}
	if ses == nil {
		return nil, fmt.Errorf("mthub: no session for account %s", req.Msg.AccountId)
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
	ses, err := s.hub.EnsureSession(req.Msg.AccountId, "")
	if err != nil {
		return nil, fmt.Errorf("mthub: ensure session: %w", err)
	}
	if ses == nil {
		return nil, fmt.Errorf("mthub: no session for account %s", req.Msg.AccountId)
	}
	conn := ses.Conn()
	if conn == nil {
		return nil, fmt.Errorf("mthub: not connected")
	}
	positions, err := mtapi.FetchOpenedOrders(ctx, conn, ses.Platform(), ses.SessionID())
	if err != nil {
		return nil, err
	}
	s.log.Info("mthub fetched positions", zap.Int("count", len(positions)))
	if len(positions) > 0 {
		s.log.Info("mthub first position", zap.Int64("ticket", positions[0].Ticket), zap.Int64("openTimeMs", positions[0].OpenTimeMs))
	}
	out := make([]*mthubv1.OrderRecord, 0, len(positions))
	for _, p := range positions {
		cp := p.CurrentPrice
		if cp == 0 || cp == p.OpenPrice {
			// Try raw symbol first, then strip suffix (EURUSDm → EURUSD)
			sym := p.Symbol
			lp := s.hub.LatestPriceForSide(sym, p.Type)
			if lp == 0 {
				if idx := len(sym) - 1; idx > 0 && (sym[idx] == 'm' || sym[idx] == 'x') {
					lp = s.hub.LatestPriceForSide(sym[:idx], p.Type)
				}
			}
			if lp > 0 {
				cp = lp
			}
		}
		out = append(out, &mthubv1.OrderRecord{
			Ticket: p.Ticket, Symbol: p.Symbol, Side: p.Type, Lots: p.Lots,
			OpenPrice: p.OpenPrice, Profit: p.Profit, Swap: p.Swap, Commission: p.Commission,
			OpenTimeMs: p.OpenTimeMs, CurrentPrice: cp,
		})
	}
	return connect.NewResponse(&mthubv1.OpenedOrdersResponse{Orders: out}), nil
}

// ── Symbol + price (stub — wired in MH-4) ──

func (s *MtHubService) SymbolParamsMany(
	ctx context.Context,
	req *connect.Request[mthubv1.SymbolParamsManyRequest],
) (*connect.Response[mthubv1.SymbolParamsManyResponse], error) {
	ses, err := s.hub.EnsureSession(req.Msg.AccountId, "")
	if err != nil {
		return nil, fmt.Errorf("mthub: ensure session: %w", err)
	}
	if ses == nil || ses.Conn() == nil {
		return nil, fmt.Errorf("mthub: not connected")
	}
	params, err := mtapi.FetchSymbolParamsMany(ctx, ses.Conn(), ses.Platform(), ses.SessionID(), req.Msg.Symbols)
	if err != nil {
		return nil, err
	}
	out := make([]*mthubv1.SymbolParam, 0, len(params))
	for _, p := range params {
		out = append(out, &mthubv1.SymbolParam{
			Symbol: p.Symbol, Digits: int32(p.Digits), Point: p.Point,
			ContractSize: p.ContractSize, MinLot: p.MinLot, MaxLot: p.MaxLot,
			LotStep: p.LotStep,
		})
	}
	return connect.NewResponse(&mthubv1.SymbolParamsManyResponse{Symbols: out}), nil
}

func (s *MtHubService) PriceHistory(
	ctx context.Context,
	req *connect.Request[mthubv1.PriceHistoryRequest],
) (*connect.Response[mthubv1.PriceHistoryResponse], error) {
	ses, err := s.hub.EnsureSession(req.Msg.AccountId, "")
	if err != nil {
		return nil, fmt.Errorf("mthub: ensure session: %w", err)
	}
	if ses == nil || ses.Conn() == nil {
		return nil, fmt.Errorf("mthub: not connected")
	}
	bars, err := mtapi.FetchPriceHistoryToday(ctx, ses.Conn(), ses.Platform(), ses.SessionID(), req.Msg.Symbol)
	if err != nil {
		return nil, err
	}
	out := make([]*mthubv1.PriceBar, 0, len(bars))
	for _, b := range bars {
		out = append(out, &mthubv1.PriceBar{
			OpenTsMs: b.Time, Open: b.Open, High: b.High,
			Low: b.Low, Close: b.Close, Volume: b.Volume,
		})
	}
	return connect.NewResponse(&mthubv1.PriceHistoryResponse{Bars: out}), nil
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
