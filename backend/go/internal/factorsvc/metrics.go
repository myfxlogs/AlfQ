// Package factorsvc — Prometheus metrics registry.
//
// Metrics per docs/15-可观测性详细规范.md §5.1:
//
//	alfq_factor_eval_total{factor, symbol, result}
//	alfq_factor_eval_duration_seconds_bucket{factor}
//	alfq_factor_loaded_count{tenant_bucket}
//	alfq_factor_dependency_depth
package factorsvc

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	FactorEvalTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alfq_factor_eval_total",
			Help: "Total factor evaluations.",
		},
		[]string{"factor", "symbol", "result"},
	)

	FactorEvalDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "alfq_factor_eval_duration_seconds",
			Help:    "Factor evaluation latency histogram.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"factor"},
	)

	FactorLoadedCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "alfq_factor_loaded_count",
			Help: "Number of loaded factor instances.",
		},
		[]string{"tenant_bucket"},
	)

	FactorDependencyDepth = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "alfq_factor_dependency_depth",
			Help:    "Factor dependency graph depth histogram.",
			Buckets: prometheus.LinearBuckets(1, 1, 10),
		},
	)
)
