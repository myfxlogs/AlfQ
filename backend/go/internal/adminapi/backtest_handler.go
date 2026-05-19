// Package adminapi — BacktestService stub handler.
package adminapi

import (
	"context"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func (s *Service) ListBacktests(ctx context.Context, req *pb.ListBacktestsRequest) (*pb.ListBacktestsResponse, error) {
	_ = req
	return &pb.ListBacktestsResponse{}, nil
}
