package bootstrap

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// registerHealthEndpoints adds /health, /healthz, /readyz, /metrics to the mux.
func registerHealthEndpoints(mux *http.ServeMux) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	// /readyz is service-specific — each register() may override.
	mux.Handle("/metrics", promhttp.Handler())
}
