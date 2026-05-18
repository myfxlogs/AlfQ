package mdgateway

import (
	"context"
	"fmt"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	mt4pb "github.com/alfq/backend/go/gen/mt4"
	"github.com/alfq/backend/go/internal/mdgateway/adapter/mt4"
)

// mt4Gateway implements Gateway using the real mtapi MT4 gRPC client.
type mt4Gateway struct {
	cfg     AccountConfig
	client  *mt4.Client
	session string
	running bool
}

func newMT4Gateway(cfg AccountConfig) Gateway {
	return &mt4Gateway{cfg: cfg}
}

func (g *mt4Gateway) Platform() string { return "mt4" }

func (g *mt4Gateway) Connect(ctx context.Context) error {
	cli, err := mt4.Dial(ctx, mt4.DefaultEndpoint)
	if err != nil {
		return fmt.Errorf("mt4 dial: %w", err)
	}
	g.client = cli

	resp, err := cli.Connection.Connect(ctx, &mt4pb.ConnectRequest{
		Host:     g.cfg.Host,
		Port:     443,
		User:     0, // TODO: parse g.cfg.Login (string) → int32
		Password: g.cfg.Password,
	})
	if err != nil {
		return fmt.Errorf("mt4 connect: %w", err)
	}
	g.session = resp.Result
	return nil
}

func (g *mt4Gateway) Disconnect(_ context.Context) error {
	g.running = false
	return nil
}

func (g *mt4Gateway) Subscribe(ctx context.Context, symbols []string, handler TickHandler) error {
	if g.client == nil {
		return fmt.Errorf("mt4: not connected")
	}
	g.running = true

	stream, err := g.client.Streams.OnQuote(ctx, &mt4pb.OnQuoteRequest{Id: g.session})
	if err != nil {
		return fmt.Errorf("mt4 subscribe: %w", err)
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

func (g *mt4Gateway) HealthCheck(ctx context.Context) error {
	if g.client == nil {
		return nil
	}
	_, err := g.client.Connection.CheckConnect(ctx, &mt4pb.CheckConnectRequest{Id: g.session})
	return err
}
