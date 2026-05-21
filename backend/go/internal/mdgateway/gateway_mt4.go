package mdgateway

import (
	"context"
	"fmt"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	mt4pb "github.com/alfq/backend/go/gen/mt4"
	"github.com/alfq/backend/go/internal/mdgateway/adapter/mt4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// mt4Gateway implements Gateway using the real mtapi MT4 gRPC client.
type mt4Gateway struct {
	cfg          AccountConfig
	client       *mt4.Client
	session      string
	running      bool
	normalizer   *Normalizer
	streamCancel context.CancelFunc
}

func newMT4Gateway(cfg AccountConfig, normalizer *Normalizer) Gateway {
	return &mt4Gateway{cfg: cfg, normalizer: normalizer}
}

func (g *mt4Gateway) Platform() string { return "mt4" }
func (g *mt4Gateway) Conn() *grpc.ClientConn {
	if g.client != nil {
		return g.client.Conn()
	}
	return nil
}
func (g *mt4Gateway) SessionID() string { return g.session }
func (g *mt4Gateway) BrokerID() string  { return g.cfg.Broker }

func (g *mt4Gateway) Connect(ctx context.Context) error {
	cli, err := mt4.Dial(ctx, mt4.DefaultEndpoint)
	if err != nil {
		return fmt.Errorf("mt4 dial: %w", err)
	}
	g.client = cli

	tempID := "mdgw-" + g.cfg.Login
	ctxWithID := metadata.AppendToOutgoingContext(ctx, "id", tempID)

	user, _ := parseUint64(g.cfg.Login)
	resp, err := cli.Connection.Connect(ctxWithID, &mt4pb.ConnectRequest{
		Host:     g.cfg.Host,
		Port:     443,
		User:     int32(user),
		Password: g.cfg.Password,
		Id:       &tempID,
	})
	if err != nil {
		return fmt.Errorf("mt4 connect: %w", err)
	}
	if resp.GetError() != nil && resp.GetError().GetMessage() != "" {
		return fmt.Errorf("mt4 connect error: %s", resp.GetError().GetMessage())
	}
	g.session = resp.GetResult()
	return nil
}

func (g *mt4Gateway) Disconnect(_ context.Context) error {
	g.running = false
	if g.streamCancel != nil {
		g.streamCancel()
	}
	return nil
}

func (g *mt4Gateway) Subscribe(ctx context.Context, symbols []string, handler TickHandler) error {
	if g.client == nil {
		return fmt.Errorf("mt4: not connected")
	}
	g.running = true

	// Use independent context for stream lifetime (caller ctx may be short-lived).
	streamCtx, streamCancel := context.WithCancel(context.Background())
	g.streamCancel = streamCancel
	mdCtx := metadata.AppendToOutgoingContext(streamCtx, "id", g.session)

	// Subscribe symbols to MarketWatch so OnQuote delivers live ticks.
	if len(symbols) > 0 {
		subReq := &mt4pb.SubscribeManyRequest{Id: g.session, Symbols: symbols}
		if _, err := g.client.Subscriptions.SubscribeMany(mdCtx, subReq); err != nil {
			return fmt.Errorf("mt4 subscribe many: %w", err)
		}
	}

	stream, err := g.client.Streams.OnQuote(mdCtx, &mt4pb.OnQuoteRequest{Id: g.session})
	if err != nil {
		return fmt.Errorf("mt4 onquote: %w", err)
	}

	go func() {
		defer func() { g.running = false }()
		for {
			reply, err := stream.Recv()
			if err != nil {
				return
			}
			q := reply.GetResult()
			if q == nil || q.Time == nil {
				continue
			}
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

func (g *mt4Gateway) normalize(tenantID, broker, symbol string, tsMs int64, bid, ask string) *pb.Tick {
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

func (g *mt4Gateway) HealthCheck(ctx context.Context) error {
	if g.client == nil {
		return nil
	}
	_, err := g.client.Connection.CheckConnect(ctx, &mt4pb.CheckConnectRequest{Id: g.session})
	return err
}
