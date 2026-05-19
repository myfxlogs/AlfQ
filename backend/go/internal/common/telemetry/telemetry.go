// Package telemetry provides OpenTelemetry and Prometheus initialization.
package telemetry

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Init starts the Prometheus metrics server and initializes OTel tracing.
// Returns a shutdown function.
func Init(ctx context.Context, log *zap.Logger, metricsPort string, serviceName string) func() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// Health endpoint for k8s probes
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Initialize OpenTelemetry tracer provider
	// In production: uses OTLP exporter to Jaeger/Tempo.
	// For development: stdout exporter or no-op.
	tpShutdown := initTracer(ctx, serviceName)

	srv := &http.Server{Addr: ":" + metricsPort, Handler: mux}
	go func() {
		log.Info("metrics server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("metrics server error", zap.Error(err))
		}
	}()
	return func() {
		srv.Shutdown(context.Background())
		if tpShutdown != nil {
			tpShutdown(ctx)
		}
	}
}

// initTracer sets up an OpenTelemetry tracer provider.
// Returns a shutdown function (nil if tracing is disabled).
func initTracer(ctx context.Context, serviceName string) func(context.Context) error {
	// TODO: Replace with real OTLP exporter to Jaeger/Tempo when infrastructure is ready.
	// Example:
	//   exp, _ := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint("jaeger:4317"))
	//   tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
	//   otel.SetTracerProvider(tp)
	_ = ctx
	_ = serviceName
	return nil
}
