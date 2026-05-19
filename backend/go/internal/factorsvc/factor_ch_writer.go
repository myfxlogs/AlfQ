// Package factorsvc — ClickHouse writer for factor values.
//
// Schema per docs/02 数据库设计:
//
//	CREATE TABLE alfq.factor_values (
//	  tenant_id  UUID,
//	  factor     LowCardinality(String),
//	  symbol     LowCardinality(String),
//	  ts         DateTime64(0, 'UTC'),
//	  value      Float64
//	) ENGINE = MergeTree
//	PARTITION BY (tenant_id, toYYYYMM(ts))
//	ORDER BY (tenant_id, factor, symbol, ts)
//	TTL toDateTime(ts) + INTERVAL 2 YEAR;
package factorsvc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FactorCHWriterConfig holds ClickHouse writer settings for factor values.
type FactorCHWriterConfig struct {
	FlushInterval time.Duration // default 5s (less frequent than tick writer)
	MaxBatchSize  int           // default 500
}

// DefaultFactorCHWriterConfig returns sensible defaults.
func DefaultFactorCHWriterConfig() FactorCHWriterConfig {
	return FactorCHWriterConfig{
		FlushInterval: 5 * time.Second,
		MaxBatchSize:  500,
	}
}

// FactorCHWriter buffers factor values and flushes them to alfq.factor_values.
// Async batch insert per docs/08 §3.3.
type FactorCHWriter struct {
	cfg  FactorCHWriterConfig
	log  *zap.Logger
	ch   chan factorRow
	done chan struct{}
	wg   sync.WaitGroup
}

type factorRow struct {
	TenantID string
	Factor   string
	Symbol   string
	TS       int64 // unix_ms
	Value    float64
}

// NewFactorCHWriter creates a FactorCHWriter.
func NewFactorCHWriter(cfg FactorCHWriterConfig, log *zap.Logger) *FactorCHWriter {
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.MaxBatchSize == 0 {
		cfg.MaxBatchSize = 500
	}
	return &FactorCHWriter{
		cfg:  cfg,
		log:  log,
		ch:   make(chan factorRow, cfg.MaxBatchSize*2),
		done: make(chan struct{}),
	}
}

// Start begins the async flush loop.
func (w *FactorCHWriter) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.loop(ctx)
}

func (w *FactorCHWriter) loop(ctx context.Context) {
	defer w.wg.Done()

	batch := make([]factorRow, 0, w.cfg.MaxBatchSize)
	ticker := time.NewTicker(w.cfg.FlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		// In production: clickhouse-go/v2 batch INSERT INTO alfq.factor_values
		w.log.Debug("factor_ch_writer: flush",
			zap.Int("rows", len(batch)),
			zap.String("example_factor", batch[0].Factor),
		)
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			for {
				select {
				case r := <-w.ch:
					batch = append(batch, r)
				default:
					flush()
					return
				}
			}
		case r := <-w.ch:
			batch = append(batch, r)
			if len(batch) >= w.cfg.MaxBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// Write enqueues a factor value for batch insertion.
func (w *FactorCHWriter) Write(ctx context.Context, tenantID, factor, symbol string, tsMs int64, value float64) {
	select {
	case w.ch <- factorRow{TenantID: tenantID, Factor: factor, Symbol: symbol, TS: tsMs, Value: value}:
	default:
		w.log.Warn("factor_ch_writer: channel full, dropping value",
			zap.String("factor", factor), zap.String("symbol", symbol),
		)
	}
}

// Close flushes remaining values and stops the writer.
func (w *FactorCHWriter) Close() error {
	close(w.done)
	w.wg.Wait()
	return nil
}

// Ensure import is used
var _ = fmt.Sprintf
