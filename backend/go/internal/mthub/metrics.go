package mthub

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// mthubActiveSessions tracks the number of active sessions by platform.
	mthubActiveSessions = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mthub_active_sessions",
		Help: "Number of active MT sessions managed by the hub, grouped by platform.",
	}, []string{"platform"})

	// mthubSessionReconnectTotal counts reconnection attempts.
	mthubSessionReconnectTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mthub_session_reconnect_total",
		Help: "Total number of session reconnection attempts, grouped by platform and reason.",
	}, []string{"platform", "reason"})

	// mthubRPCDuration tracks RPC latency by method.
	mthubRPCDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mthub_rpc_duration_seconds",
		Help:    "RPC latency in seconds, grouped by method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})

	// mthubOrderEventLag measures the delay between MT event and local processing.
	mthubOrderEventLag = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mthub_order_event_lag_seconds",
		Help: "Seconds between MT OnOrderUpdate event timestamp and local receipt.",
	})
)

func init() {
	// Initialise label dimensions so metrics appear in /metrics even before first use.
	mthubActiveSessions.WithLabelValues("mt5").Set(0)
	mthubActiveSessions.WithLabelValues("mt4").Set(0)
	mthubSessionReconnectTotal.WithLabelValues("mt5", "unknown")
	mthubSessionReconnectTotal.WithLabelValues("mt4", "unknown")
	mthubRPCDuration.WithLabelValues("EnsureSession")
	mthubRPCDuration.WithLabelValues("OrderHistory")
	mthubOrderEventLag.Set(0)
}

// recordActiveSessions updates the active session gauge.
func recordActiveSessions(active map[string]int) {
	for platform, count := range active {
		mthubActiveSessions.WithLabelValues(platform).Set(float64(count))
	}
}
