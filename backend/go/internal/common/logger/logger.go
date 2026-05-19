// Package logger provides a structured JSON logger using zap with ALFQ standard fields.
package logger

import (
	"context"

	"github.com/alfq/backend/go/internal/common/auth"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a production-ready structured logger writing JSON to stdout.
func New(level string) (*zap.Logger, error) {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return cfg.Build()
}

// WithContext returns a logger with standard ALFQ fields extracted from context.
// Injects: trace_id, tenant_id, user_id, request_id when available.
func WithContext(log *zap.Logger, ctx context.Context) *zap.Logger {
	fields := []zap.Field{}

	if tid := auth.TenantFromContext(ctx); tid != "" {
		fields = append(fields, zap.String("tenant_id", tid))
	}
	if uid := auth.UserFromContext(ctx); uid != "" {
		fields = append(fields, zap.String("user_id", uid))
	}
	if rid := requestIDFromContext(ctx); rid != "" {
		fields = append(fields, zap.String("request_id", rid))
	}

	// trace_id is extracted from OpenTelemetry span context when available
	if traceID := traceIDFromContext(ctx); traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}

	if len(fields) == 0 {
		return log
	}
	return log.With(fields...)
}

// requestIDFromContext extracts a request_id from context if set by middleware.
func requestIDFromContext(ctx context.Context) string {
	if v := ctx.Value("request_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// traceIDFromContext extracts an OpenTelemetry trace_id from context.
func traceIDFromContext(ctx context.Context) string {
	if v := ctx.Value("trace_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
