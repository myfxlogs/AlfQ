---
name: mt-quote-api
description: Use when writing or debugging MT4/MT5 gRPC calls for quotes (tick streaming), historical bars (K-line), or symbol metadata. Covers correct method names, TimeFrame values, nil-safety, and platform differences.
---

# MT4 / MT5 Quote & History API

Use this skill when calling MT4 or MT5 gRPC APIs through `gprc/mt4.proto` / `gprc/mt5.proto`. The proto stubs live in `backend/go/gen/mt[45]/`.

## 1. Session & Auth

All API calls require a session token obtained via `Connection.Connect`:

```go
connClient := mt5pb.NewConnectionClient(conn)
resp, _ := connClient.Connect(ctx, &mt5pb.ConnectRequest{
    User:     login,
    Password: password,
    Host:     host,
    Port:     port,
})
sessionID := resp.GetResult()
```

The session ID must be passed **both** as gRPC metadata (`"id"` header) **and** as the `Id` field in every request proto. The mtapi gateway requires both.

```go
ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
```

## 2. Real-Time Tick Streaming

### MT5

```
Streams.OnQuote(OnQuoteRequest{Id: session}) → stream OnQuoteReply
```

`OnQuoteReply.result` → `Quote{Symbol, Bid, Ask, Time, Last, Volume}`

### MT4

```
Streams.OnQuote(OnQuoteRequest{Id: session}) → stream OnQuoteReply
```

`OnQuoteReply.result` → `QuoteEventArgs{Symbol, Bid, Ask, Time, High, Low}`

Key differences:
- MT5: `Quote.Bid` / `Quote.Ask` (no High/Low in tick)
- MT4: `QuoteEventArgs.Bid` / `QuoteEventArgs.Ask` (has additional High/Low)

### Nil Safety

MT4 brokers may return `OnQuoteReply` with `result == nil`. **Always check:**

```go
q := reply.GetResult()
if q == nil || q.Time == nil {
    continue
}
```

### Point-In-Time Quotes (non-streaming)

| Method | MT5 | MT4 |
|--------|-----|-----|
| Single symbol | `MT5.GetQuote(id, symbol)` → `GetQuoteReply.Quote` | `MT4.Quote(id, symbol)` → `QuoteReply.QuoteEventArgs` |
| Multiple symbols | `MT5.GetQuoteMany(id, symbols[])` | `MT4.GetQuoteMany(id, symbols[])` |

## 3. Historical Bars (K-Line)

### MT5 — `QuoteHistory` Service

**Date range query — use this for backfill:**
```
QuoteHistory.PriceHistory(PriceHistoryRequest{
    Id:        sessionID,
    Symbol:    "EURUSDm",
    TimeFrame: 60,        // ← MINUTES, not MT5 PERIOD enum!
    From:      "2025-04-01T00:00:00",
    To:        "2025-05-01T00:00:00",
}) → PriceHistoryReply{Result: []*Bar}
```

**Count-based query (N bars from a point):**
```
QuoteHistory.PriceHistoryEx(PriceHistoryExRequest{
    Id:        sessionID,
    Symbol:    "EURUSDm",
    TimeFrame: 60,
    From:      "2025-05-01T00:00:00",
    NumBars:   500,
}) → PriceHistoryExReply{Result: []*Bar}
```

**Bar structure:** `Bar{Time, OpenPrice, HighPrice, LowPrice, ClosePrice, TickVolume, Spread, Volume}`

### MT4 — `QuoteHistory` Service

MT4 uses a **different method name** and an **enum type** for TimeFrame:

```
QuoteHistory.QuoteHistory(QuoteHistoryRequest{
    Id:        sessionID,
    Symbol:    "EURUSDm",
    Timeframe: H1,       // ← enum, NOT minutes! (H1, H4, D1, etc.)
    From:      "2025-04-01T00:00:00",
    Count:     500,
}) → QuoteHistoryReply{Result: []*Bar}
```

**Bar structure:** `Bar{Time, Open, High, Low, Close, TickVolume, Spread, Volume}` (note: no "Price" suffix)

## 4. TimeFrame Mapping

### MT5: int32 minutes

| Period string | TimeFrame (int32) |
|---------------|-------------------|
| `"1m"`  | 1 |
| `"5m"`  | 5 |
| `"15m"` | 15 |
| `"30m"` | 30 |
| `"1h"`  | **60** |
| `"4h"`  | **240** |
| `"1d"`  | **1440** |
| `"1w"`  | **10080** |
| `"1M"`  | **43200** |

> **Critical:** Do NOT use MT5 PERIOD enum constants (16385, 16388, 16408, 32769, 49153). These are wrong for the QuoteHistory service. Reference: `anttrader/backend/internal/service/kline_service_mt5.go:parseTimeframeMT5()`.

### MT4: enum

MT4 uses the `Timeframe` enum from the proto: `M1`, `M5`, `M15`, `M30`, `H1`, `H4`, `D1`, `W1`, `MN1`.

## 5. Symbol Names

Always use the **broker-raw** symbol name (e.g., `EURUSDm`, `XAUUSDm`), not the canonical form. MT4/MT5 brokers use suffixes like `m`, `.ecn`, `.pro`, etc. The canonical mapping happens downstream via `normalizer.go`.

## 6. Error Handling

Every reply has an `Error` field. Always check it:

```go
if e := resp.GetError(); e != nil && e.GetMessage() != "" {
    // handle error — e.GetCode() + e.GetMessage()
}
```

Common errors:
- `"EURUSD not exist"` — wrong symbol name (use raw broker name with suffix)
- `"If timeframe > 60 it should be in whole hours"` — TimeFrame value is wrong (not minutes)

## 7. Platform Differences Summary

| Concern | MT5 | MT4 |
|---------|-----|-----|
| History method | `PriceHistory` / `PriceHistoryEx` | `QuoteHistory` |
| TimeFrame type | `int32` (minutes) | `Timeframe` enum |
| Tick struct | `Quote` (no High/Low) | `QuoteEventArgs` (has High/Low) |
| Bar fields | `OpenPrice`, `HighPrice`, … | `Open`, `High`, … |
| Symbol RPC | `SymbolParamsMany`, `SymbolSessionsEx` | `SymbolParamsMany`, sessions embedded in GroupParams |
| Tick streaming | `Streams.OnQuote` | `Streams.OnQuote` |
| Nil results | Rare | **Common** — must nil-check |

## 9. Order History

### MT5 — `Trading.OrderHistory`

```
Trading.OrderHistory(OrderHistoryRequest{
    Id:   sessionID,
    From: "2024-01-01T00:00:00",
    To:   "2026-05-21T00:00:00",
}) → OrderHistoryReply{Result: []*Order}
```

**Result:** MT5 `OrderHistory` returns ~70 orders with correct date-range filtering. Symbol names include BTCUSDm with volumes in uint64 cents.

### MT4 — `Trading.OrderHistory` (LIMITED!)

```
Trading.OrderHistory(OrderHistoryRequest{
    Id:   sessionID,
    From: "2024-01-01T00:00:00",
    To:   "2026-05-21T00:00:00",
}) → OrderHistoryReply{Result: []*Order}
```

**⚠️ mtapi limitation:** MT4 `OrderHistory` returns **max 15 orders** per call. The `From`/`To` params are **ignored** — the API always returns the 15 most recent orders regardless of date range. Narrower date windows return 0 orders when none of the 15 most recent orders fall within the window.

**Workaround:** For full MT4 order history, users must export order history from MetaTrader directly and import into the system.
