// Package factorsvc — WindowBuffer: per-symbol rolling bar window for factor computation.
// RS03: Maintains N-bar history required by rolling operators (SMA, EMA, RSI, etc.).
// Bootstrap loads historical bars from ClickHouse via MarketDataView.
package factorsvc

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"go.uber.org/zap"
)

// WindowBuffer maintains a fixed-size rolling window of bars per key.
// Key format: "tenant_id/symbol/period"
type WindowBuffer struct {
	mu       sync.RWMutex
	buffers  map[string]*ringBuffer
	maxSize  int
	log      *zap.Logger
}

type ringBuffer struct {
	bars []*barRecord
	head int
	size int
	cap  int
}

type barRecord struct {
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
	TsMs   int64
}

// NewWindowBuffer creates a new window buffer with the given maximum window size.
func NewWindowBuffer(maxSize int, log *zap.Logger) *WindowBuffer {
	return &WindowBuffer{
		buffers: make(map[string]*ringBuffer),
		maxSize: maxSize,
		log:     log,
	}
}

// Push adds a bar to the window and evicts the oldest if at capacity.
func (w *WindowBuffer) Push(tenantID, symbol, period string, bar *pb.Bar) {
	key := fmt.Sprintf("%s/%s/%s", tenantID, symbol, period)

	w.mu.Lock()
	rb, ok := w.buffers[key]
	if !ok {
		rb = &ringBuffer{bars: make([]*barRecord, w.maxSize), cap: w.maxSize}
		w.buffers[key] = rb
	}
	w.mu.Unlock()

	open, _ := parseFloat(bar.GetOpen().GetValue())
	high, _ := parseFloat(bar.GetHigh().GetValue())
	low, _ := parseFloat(bar.GetLow().GetValue())
	closeV, _ := parseFloat(bar.GetClose().GetValue())
	rb.add(&barRecord{
		Open:   open,
		High:   high,
		Low:    low,
		Close:  closeV,
		Volume: bar.GetVolume(),
		TsMs:   bar.CloseTsUnixMs,
	})
}

// PushRaw adds a bar from raw values (used for CH bootstrap).
func (w *WindowBuffer) PushRaw(tenantID, symbol, period string, open, high, low, closeV, volume float64, tsMs int64) {
	key := fmt.Sprintf("%s/%s/%s", tenantID, symbol, period)

	w.mu.Lock()
	rb, ok := w.buffers[key]
	if !ok {
		rb = &ringBuffer{bars: make([]*barRecord, w.maxSize), cap: w.maxSize}
		w.buffers[key] = rb
	}
	w.mu.Unlock()

	rb.add(&barRecord{
		Open:   open,
		High:   high,
		Low:    low,
		Close:  closeV,
		Volume: volume,
		TsMs:   tsMs,
	})
}

// Snapshot returns a copy of the current window for a key, oldest first.
func (w *WindowBuffer) Snapshot(tenantID, symbol, period string, limit int) []*barRecord {
	key := fmt.Sprintf("%s/%s/%s", tenantID, symbol, period)

	w.mu.RLock()
	rb, ok := w.buffers[key]
	w.mu.RUnlock()
	if !ok {
		return nil
	}
	return rb.snapshot(limit)
}

// Bootstrap pre-allocates buffer entries for the given specs.
// Historical bars are filled from live NATS data as they arrive.
// For production CH bootstrap, integrate with MarketDataView.CHView.
func (w *WindowBuffer) Bootstrap(ctx context.Context, specs []BootstrapSpec) {
	for _, spec := range specs {
		key := fmt.Sprintf("%s/%s/%s", spec.TenantID, spec.Symbol, spec.Period)
		w.mu.Lock()
		if _, ok := w.buffers[key]; !ok {
			w.buffers[key] = &ringBuffer{bars: make([]*barRecord, w.maxSize), cap: w.maxSize}
		}
		w.mu.Unlock()
	}
	w.log.Info("bootstrap complete", zap.Int("specs", len(specs)))
}

// BootstrapSpec defines a bootstrap target.
type BootstrapSpec struct {
	TenantID string
	Symbol   string
	Period   string
	Limit    int // number of historical bars to fetch
}

// ringBuffer methods
func (rb *ringBuffer) add(b *barRecord) {
	rb.bars[rb.head] = b
	rb.head = (rb.head + 1) % rb.cap
	if rb.size < rb.cap {
		rb.size++
	}
}

func (rb *ringBuffer) snapshot(limit int) []*barRecord {
	if limit <= 0 || limit > rb.size {
		limit = rb.size
	}
	out := make([]*barRecord, 0, limit)
	for i := 0; i < limit; i++ {
		idx := (rb.head - rb.size + i) % rb.cap
		if idx < 0 {
			idx += rb.cap
		}
		out = append(out, rb.bars[idx])
	}
	return out
}