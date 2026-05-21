// Package symbolsync — MT5 symbol fetcher.
package symbolsync

import (
	"context"
	"fmt"

	mt5pb "github.com/alfq/backend/go/gen/mt5"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// FetchMT5Symbols pulls all symbols via MT5 gateway.
// SymbolParams.GetSymbolInfo() → SymbolInfo (digits, tick_size, contract_size)
// SymbolParams.GetSymbolGroup() → SymGroup (lots, swap, margin, trade_mode)
func FetchMT5Symbols(ctx context.Context, conn *grpc.ClientConn, sessionID, brokerID string, log *zap.Logger) ([]BrokerSymbol, error) {
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	client := mt5pb.NewMT5Client(conn)

	limit := int32(10000)
	resp, err := client.SymbolParamsMany(ctxWithID, &mt5pb.SymbolParamsManyRequest{Limit: &limit})
	if err != nil {
		return nil, fmt.Errorf("mt5 SymbolParamsMany: %w", err)
	}

	result := resp.GetResult()
	if len(result) == 0 {
		return nil, fmt.Errorf("mt5: no symbols returned")
	}

	var symbols []BrokerSymbol
	for _, sp := range result {
		raw := sp.GetSymbol()
		if raw == "" {
			continue
		}
		canon := Canonicalize(raw)
		sym := BrokerSymbol{BrokerID: brokerID, SymbolRaw: raw, Canonical: canon}

		if info := sp.GetSymbolInfo(); info != nil {
			sym.Digits = int16(info.GetDigits())
			sym.Point = info.GetPoints()
			sym.TickSize = info.GetTickSize()
			sym.TickValue = info.GetTickValue()
			sym.ContractSize = info.GetContractSize()
			sym.Description = info.GetDescription()
			sym.MarginCurrency = info.GetMarginCurrency()
			sym.ProfitCurrency = info.GetProfitCurrency()
		}
		if grp := sp.GetSymbolGroup(); grp != nil {
			sym.MinLot = grp.GetMinLots()
			sym.MaxLot = grp.GetMaxLots()
			sym.LotStep = grp.GetLotsStep()
			sym.MarginInitial = grp.GetInitialMargin()
			sym.SwapLong = grp.GetSwapLong()
			sym.SwapShort = grp.GetSwapShort()
			sym.TradeMode = int16(grp.GetTradeMode())
			sym.SwapMode = int16(grp.GetSwapType())
			sym.SwapRolloverDay = int16(grp.GetThreeDaysSwap())
		}

		if sym.Digits == 0 || sym.Point == 0 || sym.ContractSize == 0 {
			sym.Partial = true
		}
		symbols = append(symbols, sym)
	}

	// Sessions
	sessResp, err := client.SymbolSessionsExMany(ctxWithID, &mt5pb.SymbolSessionsExManyRequest{
		Symbol: symbolNamesMT5(result),
	})
	if err == nil {
		for i, sess := range sessResp.GetResult() {
			if i < len(symbols) {
				symbols[i].SessionsQuote = sessionsForDayToJSON(sess.GetQuotes())
				symbols[i].SessionsTrade = sessionsForDayToJSON(sess.GetTrades())
			}
		}
	} else {
		log.Warn("mt5 sessions failed", zap.Error(err))
	}

	// Timezone (float64 offset)
	tzResp, err := client.ServerTimezone(ctxWithID, &mt5pb.ServerTimezoneRequest{})
	if err == nil {
		tz := fmt.Sprintf("%+.0f", tzResp.GetResult())
		for i := range symbols {
			symbols[i].ServerTimezone = tz
		}
	} else {
		log.Warn("mt5 timezone failed", zap.Error(err))
	}

	log.Info("mt5 symbols fetched", zap.Int("count", len(symbols)), zap.String("broker_id", brokerID))
	return symbols, nil
}

func symbolNamesMT5(sps []*mt5pb.SymbolParams) []string {
	names := make([]string, 0, len(sps))
	for _, sp := range sps {
		if s := sp.GetSymbol(); s != "" {
			names = append(names, s)
		}
	}
	return names
}

func sessionsForDayToJSON(days []*mt5pb.SessionsForDay) []byte {
	if days == nil {
		return nil
	}
	type session struct {
		Start int32 `json:"start"`
		End   int32 `json:"end"`
	}
	type day struct {
		Sessions []session `json:"sessions"`
	}
	out := make([]day, len(days))
	for i, d := range days {
		ss := d.GetSessions()
		out[i].Sessions = make([]session, len(ss))
		for j, s := range ss {
			out[i].Sessions[j] = session{Start: s.GetStartTime(), End: s.GetEndTime()}
		}
	}
	return marshalJSON(out)
}
