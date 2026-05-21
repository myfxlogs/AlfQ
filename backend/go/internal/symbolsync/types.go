// Package symbolsync pulls MT4/MT5 symbol metadata into broker_symbols.
package symbolsync

import "time"

// BrokerSymbol is the unified PG row type for broker_symbols.
type BrokerSymbol struct {
	BrokerID         string
	SymbolRaw        string
	Canonical        string
	Digits           int16
	Point            float64
	TickSize         float64
	TickValue        float64
	ContractSize     float64
	MinLot           float64
	MaxLot           float64
	LotStep          float64
	MarginInitial    float64
	MarginCurrency   string
	ProfitCurrency   string
	SwapLong         float64
	SwapShort        float64
	SwapMode         int16
	SwapRolloverDay  int16
	TradeMode        int16
	Description      string
	SessionsQuote    []byte // JSONB
	SessionsTrade    []byte // JSONB
	ServerTimezone   string
	RawPayload       []byte // JSONB
	Partial          bool
	UpdatedAt        time.Time
}
