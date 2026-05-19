// Package adminapi — Service management RPC (status / restart / logs).
package adminapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// serviceConfig maps service names to their health endpoints.
var serviceConfig = []struct {
	Name      string
	Container string
	Host      string
	Port      int
}{
	{"trading-core", "deploy-trading-core-1", "trading-core", 9000},
	{"md-gateway", "deploy-md-gateway-1", "md-gateway", 9001},
	{"quant-engine", "deploy-quant-engine-1", "quant-engine", 9002},
	{"assistant-svc", "deploy-assistant-svc-1", "assistant-svc", 9003},
	{"postgres", "deploy-postgres-1", "postgres", 5432},
	{"redis", "deploy-redis-1", "redis", 6379},
	{"nats", "deploy-nats-1", "nats", 4222},
	{"clickhouse", "deploy-clickhouse-1", "clickhouse", 8123},
	{"frontend", "deploy-frontend-1", "frontend", 80},
}

func (s *Service) GetServiceStatus(ctx context.Context, _ *pb.GetServiceStatusRequest) (*pb.GetServiceStatusResponse, error) {
	var services []*pb.ServiceStatus
	for _, sc := range serviceConfig {
		st := &pb.ServiceStatus{Name: sc.Name, Container: sc.Container}
		st.Status, st.LatencyMs = checkHealth(sc.Host, sc.Port)
		services = append(services, st)
	}
	return &pb.GetServiceStatusResponse{Services: services}, nil
}

func (s *Service) RestartService(ctx context.Context, req *pb.RestartServiceRequest) (*pb.RestartServiceResponse, error) {
	name := req.Name
	for _, sc := range serviceConfig {
		if sc.Name == name {
			cmd := exec.CommandContext(ctx, "docker", "restart", sc.Container)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("restart %s: %w: %s", name, err, string(out))
			}
			return &pb.RestartServiceResponse{Name: name, Ok: true}, nil
		}
	}
	return nil, fmt.Errorf("unknown service: %s", name)
}

func (s *Service) GetServiceLogs(ctx context.Context, req *pb.GetServiceLogsRequest) (*pb.GetServiceLogsResponse, error) {
	name := req.Name
	tail := req.Tail
	if tail <= 0 {
		tail = 100
	}
	for _, sc := range serviceConfig {
		if sc.Name == name {
			args := []string{"logs", "--tail", fmt.Sprintf("%d", tail), sc.Container}
			if since := req.Since; since != "" {
				args = append(args, "--since", since)
			}
			cmd := exec.CommandContext(ctx, "docker", args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("logs %s: %w: %s", name, err, string(out))
			}
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			return &pb.GetServiceLogsResponse{Name: name, Lines: lines}, nil
		}
	}
	return nil, fmt.Errorf("unknown service: %s", name)
}

func checkHealth(host string, port int) (string, int32) {
	// Infrastructure services: TCP dial only
	if isInfra(host) {
		return checkTCP(host, port)
	}
	url := fmt.Sprintf("http://%s:%d/healthz", host, port)
	start := time.Now()
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	latency := int32(time.Since(start).Milliseconds())
	if err != nil {
		return "down", latency
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return "up", latency
	}
	return "degraded", latency
}

func isInfra(host string) bool {
	switch host {
	case "postgres", "redis", "nats", "clickhouse":
		return true
	}
	return false
}

func checkTCP(host string, port int) (string, int32) {
	addr := fmt.Sprintf("%s:%d", host, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	latency := int32(time.Since(start).Milliseconds())
	if err != nil {
		return "down", latency
	}
	conn.Close()
	return "up", latency
}
