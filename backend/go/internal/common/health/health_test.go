package health

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func TestHealthCheck(t *testing.T) {
	s := &Service{}
	resp, err := s.Check(context.Background(), connect.NewRequest(&pb.HealthCheckRequest{}))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if resp.Msg.Status != pb.HealthCheckResponse_SERVING_STATUS_SERVING {
		t.Fatalf("Status: %v", resp.Msg.Status)
	}
}
