// Package factorsvc — quant-engine factor sub-component: NATS bar subscriber + factor publisher.
package factorsvc

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"google.golang.org/protobuf/proto"
)

// Subscriber receives bar events from NATS and evaluates factors.
type Subscriber struct {
	engine   *Engine
	natsURL  string
	nc       *nats.Conn
	js       nats.JetStreamContext
	log      *zap.Logger
	chWriter *FactorCHWriter // R18: CH writer for factor values
}

// NewSubscriber creates a Subscriber.
func NewSubscriber(engine *Engine, natsURL string, log *zap.Logger) *Subscriber {
	return &Subscriber{engine: engine, natsURL: natsURL, log: log}
}

// SetCHWriter attaches a ClickHouse writer for persisting factor values (R18).
func (s *Subscriber) SetCHWriter(w *FactorCHWriter) {
	s.chWriter = w
}

// Start connects to NATS, subscribes to bar topics, and publishes factor values.
func (s *Subscriber) Start(ctx context.Context) error {
	nc, err := nats.Connect(s.natsURL)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	s.nc = nc

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("nats jetstream: %w", err)
	}
	s.js = js

	// Subscribe to all bar topics: md.bar.*.*.*
	sub, err := js.Subscribe("md.bar.>", func(msg *nats.Msg) {
		var bar pb.Bar
		if err := proto.Unmarshal(msg.Data, &bar); err != nil {
			s.log.Warn("bar unmarshal failed", zap.Error(err))
			return
		}
		s.onBar(ctx, &bar)
	}, nats.DeliverNew())
	if err != nil {
		return fmt.Errorf("subscribe md.bar: %w", err)
	}
	defer func() { _ = sub.Unsubscribe() }() //nolint:errcheck

	s.log.Info("quant-engine factor subscribed to md.bar.>")
	<-ctx.Done()
	s.nc.Close()
	return nil
}

// onBar evaluates all factors and publishes results.
// RS03: Uses engine's WindowBuffer for rolling window computation.
func (s *Subscriber) onBar(ctx context.Context, bar *pb.Bar) {
	results := s.engine.Eval(ctx, bar)

	for name, value := range results {
		subject := fmt.Sprintf("factor.%s.%s", name, bar.Symbol)
		data, err := proto.Marshal(&pb.FactorValue{
			TenantId: bar.TenantId,
			Factor:   name,
			Symbol:   bar.Symbol,
			TsUnixMs: bar.CloseTsUnixMs,
			Value:    value,
		})
		if err != nil {
			s.log.Warn("factor marshal failed", zap.String("factor", name), zap.Error(err))
			continue
		}
		if _, err := s.js.Publish(subject, data); err != nil {
			s.log.Warn("factor publish failed", zap.String("subject", subject), zap.Error(err))
		}

		// R18: Write factor values to ClickHouse
		if s.chWriter != nil {
			tenantID := bar.TenantId
			if tenantID == "" {
				tenantID = "00000000-0000-0000-0000-000000000001"
			}
			s.chWriter.Write(ctx, tenantID, name, bar.Symbol, bar.CloseTsUnixMs, value)
		} else {
			s.log.Debug("factor_ch_writer: nil writer, skipping")
		}
	}
}

// Close releases NATS resources.
func (s *Subscriber) Close() error {
	if s.nc != nil {
		s.nc.Close()
	}
	return nil
}

// Ensure import is required
var _ = fmt.Sprintf
