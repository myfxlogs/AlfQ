package mdgateway

import (
	"context"
	"fmt"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	mt5pb "github.com/alfq/backend/go/gen/mt5"
	"github.com/alfq/backend/go/internal/mdgateway/adapter/mt5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// mt5Gateway implements Gateway using the real mtapi MT5 gRPC client.
type mt5Gateway struct {
	cfg          AccountConfig
	client       *mt5.Client
	session      string
	running      bool
	normalizer   *Normalizer
	streamCancel context.CancelFunc
}

func newMT5Gateway(cfg AccountConfig, normalizer *Normalizer) Gateway {
	return &mt5Gateway{cfg: cfg, normalizer: normalizer}
}

func (g *mt5Gateway) Platform() string { return "mt5" }
func (g *mt5Gateway) Conn() *grpc.ClientConn {
	if g.client != nil {
		return g.client.Conn()
	}
	return nil
}
func (g *mt5Gateway) SessionID() string { return g.session }
func (g *mt5Gateway) BrokerID() string  { return g.cfg.Broker }

func (g *mt5Gateway) Connect(ctx context.Context) error {
	cli, err := mt5.Dial(ctx, mt5.DefaultEndpoint)
	if err != nil {
		return fmt.Errorf("mt5 dial: %w", err)
	}
	g.client = cli

	// Temp UUID for initial connect (mtapi protocol requires an id in metadata)
	tempID := "mdgw-" + g.cfg.Login
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", tempID)

	user, _ := parseUint64(g.cfg.Login)
	resp, err := cli.Connection.Connect(ctxWithID, &mt5pb.ConnectRequest{
		Host:     g.cfg.Host,
		Port:     443,
		User:     user,
		Password: g.cfg.Password,
	})
	if err != nil {
		return fmt.Errorf("mt5 connect: %w", err)
	}
	if resp.GetError() != nil && resp.GetError().GetMessage() != "" {
		return fmt.Errorf("mt5 connect error: %s", resp.GetError().GetMessage())
	}
	g.session = resp.GetResult()
	if g.session == "" {
		return fmt.Errorf("mt5 connect: empty session id (login=%s, host=%s)", g.cfg.Login, g.cfg.Host)
	}
	return nil
}

func (g *mt5Gateway) Disconnect(_ context.Context) error {
	g.running = false
	if g.streamCancel != nil {
		g.streamCancel()
	}
	return nil
}

func (g *mt5Gateway) Subscribe(ctx context.Context, symbols []string, handler TickHandler) error {
	if g.client == nil {
		return fmt.Errorf("mt5: not connected")
	}
	g.running = true

	// Use independent context for stream lifetime (caller ctx may be short-lived).
	streamCtx, streamCancel := context.WithCancel(context.Background())
	g.streamCancel = streamCancel
	mdCtx := metadata.AppendToOutgoingContext(streamCtx, "id", g.session)

	// Subscribe symbols to MarketWatch so OnQuote delivers live ticks.
	if len(symbols) > 0 {
		subReq := &mt5pb.SubscribeManyRequest{Id: g.session, Symbols: symbols}
		if _, err := g.client.Subscriptions.SubscribeMany(mdCtx, subReq); err != nil {
			return fmt.Errorf("mt5 subscribe many: %w", err)
		}
	}

	stream, err := g.client.Streams.OnQuote(mdCtx, &mt5pb.OnQuoteRequest{Id: g.session})
	if err != nil {
		return fmt.Errorf("mt5 onquote: %w", err)
	}

	go func() {
		defer func() { g.running = false }()
		tickCount := 0
		for {
			reply, err := stream.Recv()
			if err != nil {
				return
			}
			tickCount++
			if tickCount == 1 {
				// First tick after subscribe confirms stream is alive
				_ = tickCount
			}
			q := reply.GetResult()
			tsMs := q.Time.GetSeconds()*1000 + int64(q.Time.GetNanos())/1e6
			tick := g.normalize(g.cfg.TenantID, g.cfg.Broker, q.Symbol, tsMs,
				fmt.Sprintf("%.5f", q.Bid),
				fmt.Sprintf("%.5f", q.Ask))
			tick.ArrivedUnixMs = time.Now().UnixMilli()
			handler(tick)
		}
	}()

	return nil
}

func (g *mt5Gateway) normalize(tenantID, broker, symbol string, tsMs int64, bid, ask string) *pb.Tick {
	if g.normalizer != nil {
		return g.normalizer.Tick(tenantID, broker, symbol, tsMs, bid, ask)
	}
	return &pb.Tick{
		TenantId:  tenantID,
		Broker:    broker,
		Symbol:    symbol,
		Canonical: symbol,
		TsUnixMs:  tsMs,
		Bid:       &pb.Money{Value: bid},
		Ask:       &pb.Money{Value: ask},
	}
}

func (g *mt5Gateway) HealthCheck(ctx context.Context) error {
	if g.client == nil {
		return nil
	}
	mdCtx := metadata.AppendToOutgoingContext(ctx, "id", g.session)
	_, err := g.client.Connection.CheckConnect(mdCtx, &mt5pb.CheckConnectRequest{Id: g.session})
	return err
}

func parseUint64(s string) (uint64, bool) {
	var n uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint64(c-'0')
		}
	}
	return n, n > 0
}
