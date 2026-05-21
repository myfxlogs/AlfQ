// Package symbolsync — symbol conversion helpers extracted from fetchers for testability.
package symbolsync

import (
	"fmt"

	mt4pb "github.com/alfq/backend/go/gen/mt4"
	mt5pb "github.com/alfq/backend/go/gen/mt5"
)

// ConvertMT5Symbol converts a single MT5 SymbolParams to BrokerSymbol.
// Extracted from FetchMT5Symbols for unit testing.
func ConvertMT5Symbol(sp *mt5pb.SymbolParams, brokerID string, sessions []*mt5pb.SymbolSessionsEx, tz *mt5pb.ServerTimezoneReply) BrokerSymbol {
	raw := sp.GetSymbol()
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

	// Sessions for this symbol
	for _, sess := range sessions {
		if sess.GetSymbol() == raw {
			sym.SessionsQuote = sessionsForDayToJSON(sess.GetQuotes())
			sym.SessionsTrade = sessionsForDayToJSON(sess.GetTrades())
			break
		}
	}

	// Timezone
	if tz != nil {
		sym.ServerTimezone = fmt.Sprintf("%+.0f", tz.GetResult())
	}

	return sym
}

// ConvertMT4Symbol converts a single MT4 SymbolParams to BrokerSymbol.
// sessions come from SymbolInfoEx.Sessions (may be nil).
func ConvertMT4Symbol(sp *mt4pb.SymbolParams, brokerID string, sessions []*mt4pb.ConSessions, tz *mt4pb.ServerTimezoneReply) BrokerSymbol {
	raw := sp.GetSymbolName()
	canon := Canonicalize(raw)
	sym := BrokerSymbol{BrokerID: brokerID, SymbolRaw: raw, Canonical: canon}

	if info := sp.GetSymbol(); info != nil {
		sym.Digits = int16(info.GetDigits())
		sym.Point = info.GetPoint()
		sym.ContractSize = info.GetContractSize()
		sym.SwapLong = info.GetSwapLong()
		sym.SwapShort = info.GetSwapShort()
	}
	if gp := sp.GetGroupParams(); gp != nil {
		sym.MinLot = gp.GetMinLot()
		sym.MaxLot = gp.GetMaxLot()
		sym.LotStep = gp.GetLotStep()
	}

	if sym.Digits == 0 || sym.Point == 0 || sym.ContractSize == 0 {
		sym.Partial = true
	}

	// Sessions from SymbolInfoEx (one ConSessions per day, each with Quote/Trade sessions)
	sym.SessionsQuote = sessionsMT4ToJSON(sessions, true)
	sym.SessionsTrade = sessionsMT4ToJSON(sessions, false)

	// Timezone
	if tz != nil {
		sym.ServerTimezone = fmt.Sprintf("%+d", tz.GetResult()/3600)
	}

	return sym
}

// sessionsMT4ToJSON converts MT4 ConSessions to the same JSONB format as MT5.
// quote=true extracts quote sessions, quote=false extracts trade sessions.
func sessionsMT4ToJSON(sessions []*mt4pb.ConSessions, quote bool) []byte {
	if len(sessions) == 0 {
		return nil
	}
	type session struct {
		Start int32 `json:"start"`
		End   int32 `json:"end"`
	}
	type day struct {
		Sessions []session `json:"sessions"`
	}
	out := make([]day, len(sessions))
	for i, cs := range sessions {
		var conSessions []*mt4pb.ConSession
		if quote {
			conSessions = cs.GetQuote()
		} else {
			conSessions = cs.GetTrade()
		}
		out[i].Sessions = make([]session, len(conSessions))
		for j, s := range conSessions {
			out[i].Sessions[j] = session{
				Start: s.GetOpenHour()*60 + s.GetOpenMin(),
				End:   s.GetCloseHour()*60 + s.GetCloseMin(),
			}
		}
	}
	return marshalJSON(out)
}
