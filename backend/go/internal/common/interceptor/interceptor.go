// Package interceptor provides Connect/gRPC interceptor chain.
package interceptor

import (
	"context"

	"github.com/alfq/backend/go/internal/common/auth"

	"connectrpc.com/connect"
)

// NewInterceptor returns a Connect unary interceptor chain.
// Order: tenant extraction → auth → logging placeholder.
func NewInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// 1. Extract tenant from request headers
			tenantID := req.Header().Get("X-Tenant-ID")
			if tenantID != "" {
				ctx = auth.WithTenant(ctx, tenantID)
			}

			// 2. Call next in chain
			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
