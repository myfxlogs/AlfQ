// Package mdgateway — real-time data quality checks for ticks.
package mdgateway

import (
	"math"
	"sort"
	"sync"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/prometheus/client_golang/prometheus"
)

// QualityConfig holds QC thresholds.
type QualityConfig struct {
	GapMaxSeconds  float64 // max interval between consecutive ticks (default 5s)
	OutlierSigma   float64 // price deviation sigma for outlier flagging (default 5)
	SkewMaxSeconds float64 // max allowed clock skew (default 30s)
	HistorySize    int     // how many recent prices to keep for median/sigma (default 100)
}

// DefaultQualityConfig returns sensible defaults.
func DefaultQualityConfig() QualityConfig {
	return QualityConfig{
		GapMaxSeconds:  5,
		OutlierSigma:   5,
		SkewMaxSeconds: 30,
		HistorySize:    100,
	}
}

// Quality tracks per-symbol tick quality metrics.
type Quality struct {
	cfg QualityConfig

	mu     sync.Mutex
	lastTS map[string]int64     // broker:symbol → last tick ts_ms
	prices map[string][]float64 // broker:symbol → sliding window of recent prices

	// Prometheus metrics
	gapCount   *prometheus.CounterVec
	outlierCnt *prometheus.CounterVec
	skewGauge  prometheus.Gauge
}

// NewQuality creates a QC engine with registered prometheus metrics.
func NewQuality(cfg QualityConfig) *Quality {
	if cfg.HistorySize == 0 {
		cfg = DefaultQualityConfig()
	}
	q := &Quality{
		cfg:    cfg,
		lastTS: make(map[string]int64),
		prices: make(map[string][]float64),
		gapCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "md_gap_count", Help: "Tick gaps > threshold detected",
		}, []string{"broker", "symbol"}),
		outlierCnt: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "md_outlier_count", Help: "Outlier ticks detected",
		}, []string{"broker", "symbol"}),
		skewGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "md_clock_skew_seconds", Help: "Local vs broker timestamp difference",
		}),
	}
	prometheus.MustRegister(q.gapCount, q.outlierCnt, q.skewGauge)
	return q
}

// CheckResult encodes QC decisions per tick.
type CheckResult struct {
	Outlier bool // price is outlier (>sigma from median)
	Dropped bool // tick should be dropped entirely (e.g. bid > ask)
}

// Check validates a single tick. Returns QC result for logging/CH tagging.
func (q *Quality) Check(tick *pb.Tick) CheckResult {
	key := tick.Broker + ":" + tick.Symbol
	now := time.Now().UnixMilli()

	// Rule 1: bid > ask → invalid tick, drop entirely
	bid, ok := parseFloat(tick.GetBid().GetValue())
	ask, askOK := parseFloat(tick.GetAsk().GetValue())
	if ok && askOK && bid > 0 && ask > 0 && bid > ask {
		q.outlierCnt.WithLabelValues(tick.Broker, tick.Symbol).Inc()
		return CheckResult{Dropped: true}
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	var res CheckResult

	// Gap detection
	if prev, ok := q.lastTS[key]; ok {
		gap := float64(tick.TsUnixMs-prev) / 1000.0
		if gap > q.cfg.GapMaxSeconds {
			q.gapCount.WithLabelValues(tick.Broker, tick.Symbol).Inc()
		}
	}
	q.lastTS[key] = tick.TsUnixMs

	// Clock skew
	skew := math.Abs(float64(tick.TsUnixMs-now)) / 1000.0
	if skew > q.cfg.SkewMaxSeconds {
		q.skewGauge.Set(skew)
	}

	// Outlier detection
	if ok && bid > 0 {
		window := q.prices[key]
		window = append(window, bid)
		if len(window) > q.cfg.HistorySize {
			window = window[1:]
		}
		q.prices[key] = window

		if len(window) >= q.cfg.HistorySize/2 {
			median, sigma := medianSigma(window)
			if sigma > 0 && math.Abs(bid-median) > q.cfg.OutlierSigma*sigma {
				q.outlierCnt.WithLabelValues(tick.Broker, tick.Symbol).Inc()
				res.Outlier = true
			}
		}
	}

	return res
}

func medianSigma(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)

	n := len(sorted)
	var median float64
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	} else {
		median = sorted[n/2]
	}

	// Mean absolute deviation (MAD) — robust sigma estimator
	var sum float64
	for _, v := range sorted {
		sum += math.Abs(v - median)
	}
	mad := sum / float64(n)
	sigma := 1.4826 * mad // scale factor for normal distribution
	return median, sigma
}
