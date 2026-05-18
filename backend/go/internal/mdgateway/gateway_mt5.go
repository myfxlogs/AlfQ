package mdgateway

import (
	"context"
	"fmt"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	mt5pb "github.com/alfq/backend/go/gen/mt5"
	"github.com/alfq/backend/go/internal/mdgateway/adapter/mt5"
)

// mt5Gateway implements Gateway using the real mtapi MT5 gRPC client.
type mt5Gateway struct {
	cfg     AccountConfig
	client  *mt5.Client
	session string
	running bool
}

func newMT5Gateway(cfg AccountConfig) Gateway {
	return &mt5Gateway{cfg: cfg}
}

func (g *mt5Gateway) Platform() string { return "mt5" }

func (g *mt5Gateway) Connect(ctx context.Context) error {
	cli, err := mt5.Dial(ctx, mt5.DefaultEndpoint)
	if err != nil {
		return fmt.Errorf("mt5 dial: %w", err)
	}
	g.client = cli

	resp, err := cli.Connection.Connect(ctx, &mt5pb.ConnectRequest{
		Host:     g.cfg.Host,
		Port:     443,
		User:     0, // TODO: parse g.cfg.Login (string) → uint64
		Password: g.cfg.Password,
	})
	if err != nil {
		return fmt.Errorf("mt5 connect: %w", err)
	}
	g.session = resp.Result
	return nil
}

func (g *mt5Gateway) Disconnect(_ context.Context) error {
	g.running = false
	return nil
}

func (g *mt5Gateway) Subscribe(ctx context.Context, symbols []string, handler TickHandler) error {
	if g.client == nil {
		return fmt.Errorf("mt5: not connected")
	}
	g.running = true

	stream, err := g.client.Streams.OnQuote(ctx, &mt5pb.OnQuoteRequest{Id: g.session})
	if err != nil {
		return fmt.Errorf("mt5 subscribe: %w", err)
	}

	go func() {
		defer func() { g.running = false }()
		for {
			reply, err := stream.Recv()
			if err != nil {
				return
			}
			q := reply.GetResult()
			tick := &pb.Tick{
				TenantId:      "",
				Broker:        g.cfg.Broker,
				Symbol:        q.Symbol,
				TsUnixMs:      q.Time.GetSeconds()*1000 + int64(q.Time.GetNanos())/1e6,
				ArrivedUnixMs: time.Now().UnixMilli(),
				Bid:           &pb.Money{Value: fmt.Sprintf("%.5f", q.Bid)},
				Ask:           &pb.Money{Value: fmt.Sprintf("%.5f", q.Ask)},
			}
			handler(tick)
		}
	}()

	return nil
}

func (g *mt5Gateway) HealthCheck(ctx context.Context) error {
	if g.client == nil {
		return nil
	}
	_, err := g.client.Connection.CheckConnect(ctx, &mt5pb.CheckConnectRequest{Id: g.session})
	return err
}
