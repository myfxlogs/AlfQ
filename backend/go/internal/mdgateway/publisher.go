// Package mdgateway — NATS publisher.
package mdgateway

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

var (
	publishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mdgateway_publish_total",
		Help: "Total NATS publish attempts by subject prefix (tick/bar).",
	}, []string{"kind", "status"})
	publishLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mdgateway_publish_latency_seconds",
		Help:    "NATS publish latency by kind (tick/bar).",
		Buckets: prometheus.DefBuckets,
	}, []string{"kind"})
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
	start := time.Now()
	subject := fmt.Sprintf("md.tick.%s.%s", tick.Broker, tick.Symbol)
	data, err := proto.Marshal(tick)
	if err != nil {
		return fmt.Errorf("marshal tick: %w", err)
	}
	if p.js == nil {
		p.log.Warn("tick publish skipped: no JetStream",
			zap.String("subject", subject),
			zap.String("symbol", tick.Symbol),
		)
		publishTotal.WithLabelValues("tick", "error").Inc()
		return fmt.Errorf("jetstream not available")
	}
	_, err = p.js.Publish(subject, data)
	publishLatency.WithLabelValues("tick").Observe(time.Since(start).Seconds())
	if err != nil {
		p.log.Warn("tick publish failed",
			zap.String("subject", subject),
			zap.Error(err),
		)
		publishTotal.WithLabelValues("tick", "error").Inc()
	} else {
		publishTotal.WithLabelValues("tick", "ok").Inc()
	}
	return err
}

// PublishRaw sends raw bytes to any NATS subject.
func (p *Publisher) PublishRaw(subject string, data []byte) error {
	if p.nc == nil {
		return nil
	}
	return p.nc.Publish(subject, data)
}

// PublishBar converts a completed Bar to protobuf and sends it to NATS JetStream
// on subject md.bar.<broker>.<canonical>.<period>. The subscriber in quant-engine
// (factorsvc.Subscriber) expects proto-serialized pb.Bar messages via JetStream.
func (p *Publisher) PublishBar(bar Bar) error {
	if p.js == nil {
		publishTotal.WithLabelValues("bar", "error").Inc()
		return fmt.Errorf("jetstream not available")
	}
	start := time.Now()
	subject := fmt.Sprintf("md.bar.%s.%s.%s", bar.Broker, bar.Canonical, bar.Period)
	msg := &pb.Bar{
		TenantId:      bar.TenantID,
		Broker:        bar.Broker,
		Symbol:        bar.SymbolRaw,
		Canonical:     bar.Canonical,
		Period:        bar.Period,
		OpenTsUnixMs:  bar.OpenTsUnixMs,
		CloseTsUnixMs: bar.CloseTsUnixMs,
		Open:          &pb.Money{Value: strconv.FormatFloat(bar.Open, 'f', -1, 64)},
		High:          &pb.Money{Value: strconv.FormatFloat(bar.High, 'f', -1, 64)},
		Low:           &pb.Money{Value: strconv.FormatFloat(bar.Low, 'f', -1, 64)},
		Close:         &pb.Money{Value: strconv.FormatFloat(bar.Close, 'f', -1, 64)},
		Volume:        bar.Volume,
		TickCount:     int32(bar.TickCount),
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		publishTotal.WithLabelValues("bar", "error").Inc()
		return fmt.Errorf("marshal bar: %w", err)
	}
	_, err = p.js.Publish(subject, data)
	publishLatency.WithLabelValues("bar").Observe(time.Since(start).Seconds())
	if err != nil {
		publishTotal.WithLabelValues("bar", "error").Inc()
	} else {
		publishTotal.WithLabelValues("bar", "ok").Inc()
	}
	return err
}

// EnsureStreams creates JetStream streams for tick and bar persistence
// (idempotent — safe to call on every startup).
func (p *Publisher) EnsureStreams(log *zap.Logger) {
	if p.js == nil {
		return
	}
	streams := []struct {
		name     string
		subjects []string
		maxAge   time.Duration
		maxBytes int64
	}{
		{"MD_TICKS", []string{"md.tick.>"}, 24 * time.Hour, 8 * 1024 * 1024 * 1024},
		{"MD_BARS", []string{"md.bar.>"}, 30 * 24 * time.Hour, 4 * 1024 * 1024 * 1024},
		{"FACTOR_VALUES", []string{"factor.>"}, 7 * 24 * time.Hour, 2 * 1024 * 1024 * 1024},
		{"SIGNALS", []string{"signal.>"}, 30 * 24 * time.Hour, 256 * 1024 * 1024},
	}
	for _, s := range streams {
		_, err := p.js.AddStream(&nats.StreamConfig{
			Name:     s.name,
			Subjects: s.subjects,
			Storage:  nats.FileStorage,
			MaxAge:   s.maxAge,
			MaxBytes: s.maxBytes,
		})
		if err != nil {
			log.Warn("nats add stream failed", zap.String("stream", s.name), zap.Error(err))
		} else {
			log.Info("nats stream ensured", zap.String("stream", s.name))
		}
	}
}

// Close releases NATS resources.
func (p *Publisher) Close() error {
	if p.nc != nil {
		p.nc.Close()
	}
	return nil
}
