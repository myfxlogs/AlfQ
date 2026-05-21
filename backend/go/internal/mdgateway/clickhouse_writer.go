// Package mdgateway — ClickHouse async batch writer.
package mdgateway

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// CHWriterConfig holds ClickHouse writer settings.
type CHWriterConfig struct {
	FlushInterval time.Duration // batch flush interval (default 1s)
	MaxBatchSize  int           // max rows per batch (default 1000)
}

// DefaultCHWriterConfig returns sensible defaults.
func DefaultCHWriterConfig() CHWriterConfig {
	return CHWriterConfig{
		FlushInterval: time.Second,
		MaxBatchSize:  1000,
	}
}

// CHWriter buffers ticks and flushes them to ClickHouse in batches.
type CHWriter struct {
	cfg    CHWriterConfig
	log    *zap.Logger
	chConn *CHConn
	ticks  chan *pb.Tick
	done   chan struct{}
	wg     sync.WaitGroup
}

// NewCHWriter creates a CHWriter backed by a real ClickHouse connection.
func NewCHWriter(cfg CHWriterConfig, chConn *CHConn, log *zap.Logger) *CHWriter {
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.MaxBatchSize == 0 {
		cfg.MaxBatchSize = 1000
	}
	return &CHWriter{
		cfg:    cfg,
		log:    log,
		chConn: chConn,
		ticks:  make(chan *pb.Tick, cfg.MaxBatchSize*2),
		done:   make(chan struct{}),
	}
}

// Start begins the async flush loop.
func (w *CHWriter) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.loop(ctx)
}

// loop is the main flush loop.
func (w *CHWriter) loop(ctx context.Context) {
	defer w.wg.Done()

	batch := make([]*pb.Tick, 0, w.cfg.MaxBatchSize)
	ticker := time.NewTicker(w.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.flushBatch(context.Background(), batch)
			return
		case <-w.done:
			w.flushBatch(context.Background(), batch)
			return
		case t := <-w.ticks:
			batch = append(batch, t)
			if len(batch) >= w.cfg.MaxBatchSize {
				w.flushBatch(ctx, batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				w.flushBatch(ctx, batch)
				batch = batch[:0]
			}
		}
	}
}

// Write enqueues a Tick for batch insertion. Non-blocking — drops on full channel.
func (w *CHWriter) Write(tick *pb.Tick) {
	select {
	case w.ticks <- tick:
	default:
		w.log.Warn("chwriter: channel full, dropping tick",
			zap.String("symbol", tick.Symbol),
		)
	}
}

// flushBatch writes a batch of ticks to ClickHouse.
func (w *CHWriter) flushBatch(ctx context.Context, batch []*pb.Tick) {
	if len(batch) == 0 {
		return
	}

	conn, err := w.chConn.Conn(ctx)
	if err != nil {
		w.log.Error("chwriter: get conn", zap.Error(err))
		return
	}

	insCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	chBatch, err := conn.PrepareBatch(insCtx, "INSERT INTO md_ticks")
	if err != nil {
		w.log.Error("chwriter: prepare batch", zap.Error(err))
		return
	}

	for _, t := range batch {
		// Pass bid/ask as strings to preserve decimal precision (CH driver handles
		// string→Decimal(18,6) conversion natively).
		// canonical column currently mirrors symbol_raw — resolved via broker_symbols
		// lookup once per-broker canonical mapping is available upstream.
		if err := chBatch.Append(
			t.TenantId,
			t.Broker,
			t.Symbol,    // symbol_raw
			t.Canonical, // canonical (filled by normalizer)
			uint64(t.TsUnixMs),
			uint64(t.ArrivedUnixMs),
			t.GetBid().GetValue(), // Decimal(18,6) from string
			t.GetAsk().GetValue(),
			t.BidVolume,
			t.AskVolume,
		); err != nil {
			w.log.Error("chwriter: append", zap.Error(err))
			// Continue with remaining ticks in batch
		}
	}

	if err := chBatch.Send(); err != nil {
		w.log.Error("chwriter: send batch", zap.Error(err), zap.Int("rows", len(batch)))
		return
	}

	w.log.Debug("chwriter: flushed batch", zap.Int("rows", len(batch)))
}

// Close flushes remaining ticks and closes the writer.
func (w *CHWriter) Close() error {
	close(w.done)
	w.wg.Wait()
	return nil
}
