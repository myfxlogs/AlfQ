// Package symbolsync — MT4 symbol fetcher.
package symbolsync

import (
	"context"
	"fmt"
	"sync"
	"time"

	mt4pb "github.com/alfq/backend/go/gen/mt4"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	mt4Concurrency    = 10
	mt4PerCallTimeout = 10 * time.Second
)

// FetchMT4Symbols pulls all symbols via MT4 gateway.
// Uses Symbols → list names, then concurrent SymbolParams per symbol for full metadata.
// Per-symbol SymbolParams returns SymbolInfoEx.Sessions; SymbolParamsMany may not.
// Reference: anttrader/backend/internal/mt4client/account_methods.go
func FetchMT4Symbols(ctx context.Context, conn *grpc.ClientConn, sessionID, brokerID string, log *zap.Logger) ([]BrokerSymbol, error) {
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", sessionID)
	client := mt4pb.NewMT4Client(conn)

	// Step 1: get symbol names
	symsResp, err := client.Symbols(ctxWithID, &mt4pb.SymbolsRequest{Id: sessionID})
	if err != nil {
		return fetchMT4ViaSymbolParamsMany(ctxWithID, client, sessionID, brokerID, log)
	}
	names := symsResp.GetResult()
	if len(names) == 0 {
		return nil, fmt.Errorf("mt4: no symbols returned")
	}

	log.Info("mt4 symbol names fetched", zap.Int("count", len(names)))

	// Step 2: concurrent per-symbol SymbolParams
	type result struct {
		sym BrokerSymbol
		ok  bool
	}
	sem := make(chan struct{}, mt4Concurrency)
	results := make(chan result, len(names))
	var wg sync.WaitGroup

	for _, name := range names {
		wg.Add(1)
		go func(symbolName string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			callCtx, cancel := context.WithTimeout(ctx, mt4PerCallTimeout)
			defer cancel()
			callCtx = metadata.AppendToOutgoingContext(callCtx, "id", sessionID)

			spResp, err := client.SymbolParams(callCtx, &mt4pb.SymbolParamsRequest{
				Id:     sessionID,
				Symbol: symbolName,
			})
			if err != nil || spResp == nil {
				return
			}
			if e := spResp.GetError(); e != nil && e.GetMessage() != "" {
				return
			}
			sp := spResp.GetResult()
			if sp == nil {
				return
			}

			var mt4Sessions []*mt4pb.ConSessions
			if info := sp.GetSymbol(); info != nil {
				if ex := info.GetEx(); ex != nil {
					mt4Sessions = ex.GetSessions()
				}
			}
			sym := ConvertMT4Symbol(sp, brokerID, mt4Sessions, nil)
			results <- result{sym: sym, ok: true}
		}(name)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var symbols []BrokerSymbol
	withSessions := 0
	for r := range results {
		if r.ok {
			symbols = append(symbols, r.sym)
			if r.sym.SessionsQuote != nil {
				withSessions++
			}
		}
	}

	log.Info("mt4 symbols converted",
		zap.Int("count", len(symbols)),
		zap.Int("with_sessions", withSessions),
	)

	// Timezone
	tzResp, err := client.ServerTimezone(ctxWithID, &mt4pb.ServerTimezoneRequest{})
	if err == nil {
		tz := fmt.Sprintf("%+d", tzResp.GetResult()/3600)
		for i := range symbols {
			symbols[i].ServerTimezone = tz
		}
	} else {
		log.Warn("mt4 timezone failed", zap.Error(err))
	}

	log.Info("mt4 symbols fetch complete", zap.Int("count", len(symbols)), zap.String("broker_id", brokerID))
	return symbols, nil
}

func fetchMT4ViaSymbolParamsMany(ctx context.Context, client mt4pb.MT4Client, sessionID, brokerID string, log *zap.Logger) ([]BrokerSymbol, error) {
	resp, err := client.SymbolParamsMany(ctx, &mt4pb.SymbolParamsManyRequest{})
	if err != nil {
		return nil, fmt.Errorf("mt4 SymbolParamsMany: %w", err)
	}

	result := resp.GetResult()
	if len(result) == 0 {
		return nil, fmt.Errorf("mt4: no symbols returned")
	}

	var symbols []BrokerSymbol
	for _, sp := range result {
		if sp.GetSymbolName() == "" {
			continue
		}
		var mt4Sessions []*mt4pb.ConSessions
		if info := sp.GetSymbol(); info != nil {
			if ex := info.GetEx(); ex != nil {
				mt4Sessions = ex.GetSessions()
			}
		}
		sym := ConvertMT4Symbol(sp, brokerID, mt4Sessions, nil)
		symbols = append(symbols, sym)
	}

	return symbols, nil
}
