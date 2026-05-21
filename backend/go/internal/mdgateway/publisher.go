// Package mdgateway — NATS publisher.
package mdgateway

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Publisher sends normalized ticks to NATS JetStream.
type Publisher struct {
	log     *zap.Logger
	natsURL string
	nc      *nats.Conn
	js      nats.JetStreamContext
}

// NewPublisher creates a Publisher.
func NewPublisher(log *zap.Logger, natsURL string) *Publisher {
	return &Publisher{log: log, natsURL: natsURL}
}

// Connect initializes the NATS connection and JetStream context.
func (p *Publisher) Connect(ctx context.Context) error {
	nc, err := nats.Connect(p.natsURL)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	p.nc = nc

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("nats jetstream: %w", err)
	}
	p.js = js
	return nil
}

// Publish sends a Tick to NATS subject md.tick.<broker>.<symbol>.
func (p *Publisher) Publish(ctx context.Context, tick *pb.Tick) error {
	subject := fmt.Sprintf("md.tick.%s.%s", tick.Broker, tick.Symbol)
	data, err := proto.Marshal(tick)
	if err != nil {
		return fmt.Errorf("marshal tick: %w", err)
	}
	if p.js == nil {
		p.log.Debug("tick (no nats)",
			zap.String("subject", subject),
			zap.String("symbol", tick.Symbol),
		)
		return nil
	}
	_, err = p.js.Publish(subject, data)
	return err
}

// PublishRaw sends raw bytes to any NATS subject.
func (p *Publisher) PublishRaw(subject string, data []byte) error {
	if p.nc == nil {
		return nil
	}
	return p.nc.Publish(subject, data)
}

// Close releases NATS resources.
func (p *Publisher) Close() error {
	if p.nc != nil {
		p.nc.Close()
	}
	return nil
}
