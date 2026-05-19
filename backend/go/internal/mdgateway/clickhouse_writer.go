// Package mdgateway — ClickHouse async batch writer.
package mdgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// CHWriterConfig holds ClickHouse writer settings.
type CHWriterConfig struct {
	FlushInterval time.Duration // batch flush interval (default 1s)
	MaxBatchSize  int           // max rows per batch (default 1000)
	SpillDir      string        // disk buffer directory for failed writes
}

// DefaultCHWriterConfig returns sensible defaults.
func DefaultCHWriterConfig() CHWriterConfig {
	return CHWriterConfig{
		FlushInterval: time.Second,
		MaxBatchSize:  1000,
		SpillDir:      "/tmp/alfq-ch-spill",
	}
}

// CHWriter buffers ticks and flushes them to ClickHouse in batches.
//
// In production, this uses github.com/ClickHouse/clickhouse-go/v2:
//
//	conn, _ := clickhouse.Open(&clickhouse.Options{Addr: []string{"localhost:9000"}})
//	batch, _ := conn.PrepareBatch(ctx, "INSERT INTO md_ticks")
//	batch.Append(...)
//	batch.Send()
//
// Until clickhouse-go is available in the module cache, this implementation
// writes JSON Lines to disk as a backpressure-safe fallback.
type CHWriter struct {
	cfg   CHWriterConfig
	log   *zap.Logger
	ticks chan *pb.Tick
	done  chan struct{}
	wg    sync.WaitGroup
	file  *os.File
	mu    sync.Mutex
	seq   int64
}

// NewCHWriter creates a CHWriter.
func NewCHWriter(cfg CHWriterConfig, log *zap.Logger) (*CHWriter, error) {
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.MaxBatchSize == 0 {
		cfg.MaxBatchSize = 1000
	}
	if cfg.SpillDir == "" {
		cfg.SpillDir = "/tmp/alfq-ch-spill"
	}
	if err := os.MkdirAll(cfg.SpillDir, 0o755); err != nil {
		return nil, fmt.Errorf("chwriter: mkdir %s: %w", cfg.SpillDir, err)
	}

	w := &CHWriter{
		cfg:   cfg,
		log:   log,
		ticks: make(chan *pb.Tick, cfg.MaxBatchSize*2),
		done:  make(chan struct{}),
	}
	return w, nil
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

	flush := func() {
		if len(batch) == 0 {
			return
		}
		w.flushBatch(batch)
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			// Drain remaining ticks before exit.
			for {
				select {
				case t := <-w.ticks:
					batch = append(batch, t)
				default:
					flush()
					return
				}
			}

		case t := <-w.ticks:
			batch = append(batch, t)
			if len(batch) >= w.cfg.MaxBatchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// Write enqueues a Tick for batch insertion.
func (w *CHWriter) Write(tick *pb.Tick) {
	select {
	case w.ticks <- tick:
	default:
		// Backpressure: channel full — spill to disk.
		w.mu.Lock()
		w.spillToDisk(tick)
		w.mu.Unlock()
	}
}

// flushBatch writes a batch of ticks to ClickHouse (via spill file for now).
func (w *CHWriter) flushBatch(batch []*pb.Tick) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		f, err := os.Create(fmt.Sprintf("%s/ch-batch-%d.jsonl", w.cfg.SpillDir, w.seq))
		if err != nil {
			w.log.Error("chwriter: create spill file", zap.Error(err))
			return
		}
		w.file = f
		w.seq++
	}

	enc := json.NewEncoder(w.file)
	for _, t := range batch {
		if err := enc.Encode(tickToJSON(t)); err != nil {
			w.log.Error("chwriter: encode tick", zap.Error(err))
		}
	}

	// Rotate spill file every 10 batches to keep files manageable.
	w.seq++
	if w.seq%10 == 0 {
		w.file.Close()
		w.file = nil
	}

	w.log.Debug("chwriter: flushed batch", zap.Int("rows", len(batch)))
}

// spillToDisk writes a single tick to a dedicated overflow file.
func (w *CHWriter) spillToDisk(tick *pb.Tick) {
	f, err := os.OpenFile(w.cfg.SpillDir+"/overflow.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		w.log.Error("chwriter: spill overflow", zap.Error(err))
		return
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(tickToJSON(tick)); err != nil {
		w.log.Error("chwriter: encode overflow", zap.Error(err))
	}
}

// Close flushes remaining ticks and closes the writer.
func (w *CHWriter) Close() error {
	close(w.done)
	w.wg.Wait()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}
	return nil
}

// tickToJSON converts a Tick to a flat JSON map suitable for ClickHouse.
func tickToJSON(t *pb.Tick) map[string]any {
	return map[string]any{
		"tenant_id":       t.TenantId,
		"broker":          t.Broker,
		"symbol":          t.Symbol,
		"ts_unix_ms":      t.TsUnixMs,
		"arrived_unix_ms": t.ArrivedUnixMs,
		"bid":             t.GetBid().GetValue(),
		"ask":             t.GetAsk().GetValue(),
		"bid_volume":      t.BidVolume,
		"ask_volume":      t.AskVolume,
	}
}
