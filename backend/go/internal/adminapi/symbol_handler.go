// Package adminapi — SymbolService RPC handler implementations.
package adminapi

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/gen/alfq/v1/alfqv1connect"
)

// symbolServiceHandler implements alfqv1connect.SymbolServiceHandler.
type symbolServiceHandler struct {
	svc *Service
}

// NewSymbolServiceHandler creates a ConnectRPC handler for SymbolService.
func NewSymbolServiceHandler(svc *Service) (string, http.Handler) {
	return alfqv1connect.NewSymbolServiceHandler(&symbolServiceHandler{svc: svc})
}

func (h *symbolServiceHandler) ListSymbols(ctx context.Context, req *connect.Request[pb.ListSymbolsRequest]) (*connect.Response[pb.ListSymbolsResponse], error) {
	if err := RequireTenant(ctx); err != nil {
		return nil, err
	}
	// ListSymbols returns canonical symbol names from broker_symbols
	rows, err := h.svc.pool.Query(ctx, `
		SELECT DISTINCT canonical, COALESCE(description,''), digits, COALESCE(contract_size,0), COALESCE(profit_currency,'')
		FROM broker_symbols
		WHERE broker_id = $1
		ORDER BY canonical
	`, req.Msg.BrokerId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []*pb.Symbol
	for rows.Next() {
		s := &pb.Symbol{}
		if err := rows.Scan(&s.Name, &s.Description, &s.Digits, &s.ContractSize, &s.Currency); err != nil {
			continue
		}
		symbols = append(symbols, s)
	}
	return connect.NewResponse(&pb.ListSymbolsResponse{Symbols: symbols}), rows.Err()
}

func (h *symbolServiceHandler) ListBrokerSymbols(ctx context.Context, req *connect.Request[pb.ListBrokerSymbolsRequest]) (*connect.Response[pb.ListBrokerSymbolsResponse], error) {
	if err := RequireTenant(ctx); err != nil {
		return nil, err
	}

	rows, err := h.svc.pool.Query(ctx, `
		SELECT broker_id::text, symbol_raw, canonical,
		       digits, COALESCE(point,0), COALESCE(tick_size,0),
		       COALESCE(contract_size,0), COALESCE(min_lot,0), COALESCE(max_lot,0), COALESCE(lot_step,0),
		       COALESCE(swap_long,0), COALESCE(swap_short,0),
		       COALESCE(description,'')
		FROM broker_symbols
		WHERE broker_id = $1
		ORDER BY symbol_raw
	`, req.Msg.BrokerId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []*pb.BrokerSymbolInfo
	for rows.Next() {
		s := &pb.BrokerSymbolInfo{}
		if err := rows.Scan(
			&s.BrokerId, &s.SymbolRaw, &s.Canonical,
			&s.Digits, &s.Point, &s.TickSize,
			&s.ContractSize, &s.MinLot, &s.MaxLot, &s.LotStep,
			&s.SwapLong, &s.SwapShort,
			&s.Description,
		); err != nil {
			continue
		}
		symbols = append(symbols, s)
	}
	return connect.NewResponse(&pb.ListBrokerSymbolsResponse{Symbols: symbols}), rows.Err()
}

func (h *symbolServiceHandler) LookupSymbol(ctx context.Context, req *connect.Request[pb.LookupSymbolRequest]) (*connect.Response[pb.BrokerSymbolInfo], error) {
	if err := RequireTenant(ctx); err != nil {
		return nil, err
	}

	s := &pb.BrokerSymbolInfo{}
	err := h.svc.pool.QueryRow(ctx, `
		SELECT broker_id::text, symbol_raw, canonical,
		       digits, COALESCE(point,0), COALESCE(tick_size,0),
		       COALESCE(contract_size,0), COALESCE(min_lot,0), COALESCE(max_lot,0), COALESCE(lot_step,0),
		       COALESCE(swap_long,0), COALESCE(swap_short,0),
		       COALESCE(description,'')
		FROM broker_symbols
		WHERE canonical = $1 AND broker_id = $2
		ORDER BY updated_at DESC LIMIT 1
	`, req.Msg.Canonical, req.Msg.BrokerId,
	).Scan(
		&s.BrokerId, &s.SymbolRaw, &s.Canonical,
		&s.Digits, &s.Point, &s.TickSize,
		&s.ContractSize, &s.MinLot, &s.MaxLot, &s.LotStep,
		&s.SwapLong, &s.SwapShort,
		&s.Description,
	)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(s), nil
}
