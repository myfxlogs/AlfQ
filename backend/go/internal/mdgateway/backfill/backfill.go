// Package backfill provides historical bar fetching from MT5.
package backfill

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alfq/backend/go/internal/common/config"
	mt5pb "github.com/alfq/backend/go/gen/mt5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// TimeFrame maps period string to minutes for the QuoteHistory PriceHistory API.
// Note: the MT5 QuoteHistory service uses minutes (not the MT5 PERIOD enum constants)
// for its TimeFrame parameter. See anttrader reference: parseTimeframeMT5().
var periodToTF = map[string]int32{
	"1m":  1,
	"5m":  5,
	"15m": 15,
	"30m": 30,
	"1h":  60,
	"4h":  240,
	"1d":  1440,
	"1w":  10080,
	"1M":  43200,
}

// Session holds a connected MT5 session.
type Session struct {
	conn      *grpc.ClientConn
	sessionID string
	qh        mt5pb.QuoteHistoryClient
}

// Connect establishes a gRPC connection and authenticates.
func Connect(ctx context.Context, gw config.GatewayConfig, login, password, server string) (*Session, error) {
	timeout := gw.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var creds credentials.TransportCredentials
	if gw.UseTLS {
		creds = credentials.NewTLS(&tls.Config{})
	} else {
		creds = insecure.NewCredentials()
	}
	conn, err := grpc.DialContext(dialCtx, gw.Addr,
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("backfill: dial: %w", err)
	}

	tempID := uuid.New().String()
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", tempID)

	host, port := splitHostPort(server, "443")
	connClient := mt5pb.NewConnectionClient(conn)
	resp, err := connClient.Connect(ctxWithID, &mt5pb.ConnectRequest{
		User:     parseUint(login),
		Password: password,
		Host:     host,
		Port:     int32(parsePort(port)),
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("backfill: connect: %w", err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		conn.Close()
		return nil, fmt.Errorf("backfill: mt5 error: %s", e.GetMessage())
	}

	return &Session{
		conn:      conn,
		sessionID: resp.GetResult(),
		qh:        mt5pb.NewQuoteHistoryClient(conn),
	}, nil
}

// Close releases the session.
func (s *Session) Close() error { return s.conn.Close() }

// Bar is a single OHLCV bar returned from MT5.
type Bar struct {
	Time   int64   // unix_ms
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// FetchBars pulls historical bars using PriceHistory (From + To range).
func (s *Session) FetchBars(ctx context.Context, symbol, period string, from, to time.Time, log *zap.Logger) ([]Bar, error) {
	tf, ok := periodToTF[period]
	if !ok {
		return nil, fmt.Errorf("backfill: unknown period %q", period)
	}

	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", s.sessionID)
	phClient := mt5pb.NewQuoteHistoryClient(s.conn)

	fromStr := from.Format("2006-01-02T15:04:05")
	toStr := to.Format("2006-01-02T15:04:05")

	log.Debug("backfill request",
		zap.String("symbol", symbol),
		zap.String("period", period),
		zap.String("from", fromStr),
		zap.String("to", toStr),
	)

	resp, err := phClient.PriceHistory(ctxWithID, &mt5pb.PriceHistoryRequest{
		Id:        s.sessionID,
		Symbol:    symbol,
		TimeFrame: tf,
		From:      fromStr,
		To:        toStr,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no data") {
			return nil, nil
		}
		return nil, fmt.Errorf("backfill: PriceHistory %s/%s [%s→%s]: %w",
			symbol, period, fromStr, toStr, err)
	}
	if e := resp.GetError(); e != nil && e.GetMessage() != "" {
		log.Warn("PriceHistory returned error",
			zap.String("symbol", symbol),
			zap.String("period", period),
			zap.String("from", fromStr),
			zap.String("to", toStr),
			zap.Int32("code", int32(e.GetCode())),
			zap.String("msg", e.GetMessage()),
		)
		return nil, nil
	}

	var allBars []Bar
	for _, r := range resp.GetResult() {
		ts := r.GetTime()
		ms := ts.GetSeconds()*1000 + int64(ts.GetNanos()/1e6)
		allBars = append(allBars, Bar{
			Time:   ms,
			Open:   r.GetOpenPrice(),
			High:   r.GetHighPrice(),
			Low:    r.GetLowPrice(),
			Close:  r.GetClosePrice(),
			Volume: float64(r.GetTickVolume()),
		})
	}

	log.Debug("backfill result",
		zap.String("symbol", symbol),
		zap.String("period", period),
		zap.Int("bars", len(allBars)),
	)

	return allBars, nil
}

func splitHostPort(hostPort, defaultPort string) (string, string) {
	for i := len(hostPort) - 1; i >= 0; i-- {
		if hostPort[i] == ':' {
			return hostPort[:i], hostPort[i+1:]
		}
	}
	return hostPort, defaultPort
}

func parseUint(s string) uint64 {
	var n uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint64(c-'0')
		}
	}
	return n
}

func parsePort(s string) int {
	n, _ := strconv.Atoi(s)
	if n == 0 {
		return 443
	}
	return n
}