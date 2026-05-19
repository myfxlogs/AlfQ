// Package adminapi — AuditService stub handler.
package adminapi

import (
	"context"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func (s *Service) ListAuditLogs(ctx context.Context, req *pb.ListAuditLogsRequest) (*pb.ListAuditLogsResponse, error) {
	_ = req
	return &pb.ListAuditLogsResponse{}, nil
}
