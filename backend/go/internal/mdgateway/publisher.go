// Package mdgateway — NATS publisher.
package mdgateway

import (
	"context"
	"fmt"
	"go.uber.org/zap"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Publisher sends normalized ticks to NATS and ClickHouse.
type Publisher struct {
	log     *zap.Logger
	natsURL string
}

// NewPublisher creates a Publisher.
func NewPublisher(log *zap.Logger, natsURL string) *Publisher {
	return &Publisher{log: log, natsURL: natsURL}
}

// Publish sends a Tick to NATS subject md.tick.<broker>.<symbol>.
//
// In production this uses github.com/nats-io/nats.go JetStream:
//
//	js, _ := jetstream.New(nc)
//	js.Publish(ctx, fmt.Sprintf("md.tick.%s.%s", tick.Broker, tick.Symbol), data)
func (p *Publisher) Publish(ctx context.Context, tick *pb.Tick) error {
	subject := fmt.Sprintf("md.tick.%s.%s", tick.Broker, tick.Symbol)
	p.log.Debug("tick",
		zap.String("subject", subject),
		zap.String("symbol", tick.Symbol),
		zap.String("bid", tick.GetBid().GetValue()),
		zap.String("ask", tick.GetAsk().GetValue()),
	)

	// TODO: NATS connect + JetStream publish
	_ = subject
	_ = ctx
	return nil
}

// Close releases NATS resources.
func (p *Publisher) Close() error {
	return nil
}
