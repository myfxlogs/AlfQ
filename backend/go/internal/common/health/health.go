// Package health implements the HealthService.
package health

import (
	"context"

	"connectrpc.com/connect"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/gen/alfq/v1/alfqv1connect"
)

// Service implements the HealthServiceHandler interface.
type Service struct{}

// Ensure Service implements the interface.
var _ alfqv1connect.HealthServiceHandler = (*Service)(nil)

// Check returns the current serving status.
func (s *Service) Check(_ context.Context, req *connect.Request[pb.HealthCheckRequest]) (*connect.Response[pb.HealthCheckResponse], error) {
	return connect.NewResponse(&pb.HealthCheckResponse{
		Status: pb.HealthCheckResponse_SERVING_STATUS_SERVING,
	}), nil
}
