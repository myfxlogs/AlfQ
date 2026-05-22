// Package accountconn — shared types (formerly in mtapi, now local per RS08).
package accountconn

// AccountSummaryInfo holds a full MT account summary from the broker.
type AccountSummaryInfo struct {
	Balance     float64
	Equity      float64
	Margin      float64
	FreeMargin  float64
	MarginLevel float64
	Profit      float64
	Currency    string
	Leverage    int32
	AccountType string // "demo", "real", "contest"
	IsInvestor  bool
}

// PositionInfo is a unified open position record from MT4/MT5.
type PositionInfo struct {
	Ticket       int64
	Symbol       string
	Type         string // "buy" | "sell"
	Lots         float64
	OpenPrice    float64
	Profit       float64
	Swap         float64
	Commission   float64
	OpenTimeMs   int64   // position open timestamp (UTC ms)
	CurrentPrice float64 // latest bid/ask
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
	CloseTime  string // RFC3339 or empty; empty when not closed
}
