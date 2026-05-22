// Package adminapi — AI Assistant handlers: usage stats + API key test (R10).
// Both are now proper Connect RPC handlers registered via SystemSettingsService.
package adminapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/alfq/backend/go/internal/common/auth"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// GetAIUsageStats returns per-user AI usage data as a Connect RPC.
func (s *Service) GetAIUsageStats(ctx context.Context, req *connect.Request[pb.GetAIUsageStatsRequest]) (*connect.Response[pb.GetAIUsageStatsResponse], error) {
	userID := auth.UserFromContext(ctx)
	if userID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("未登录"))
	}

	stats, err := s.getAIUsageStats(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("usage stats: %w", err))
	}

	return connect.NewResponse(&pb.GetAIUsageStatsResponse{
		Stats: stats,
	}), nil
}

// TestAPIKey tests connectivity for a given provider's API key as a Connect RPC.
func (s *Service) TestAPIKey(ctx context.Context, req *connect.Request[pb.TestAPIKeyRequest]) (*connect.Response[pb.TestAPIKeyResponse], error) {
	userID := auth.UserFromContext(ctx)
	if userID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("未登录"))
	}

	provider := req.Msg.Provider
	if provider == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("缺少 provider"))
	}

	// Read encrypted key from DB
	var keyCipher string
	err := s.pool.QueryRow(ctx,
		`SELECT key_cipher FROM user_api_keys WHERE user_id=$1 AND provider=$2`,
		userID, provider).Scan(&keyCipher)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("未设置 %s API Key", provider))
	}

	if s.encCipher == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("加密未配置"))
	}

	apiKey, err := s.encCipher.Decrypt(keyCipher)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("解密失败"))
	}

	// Test connectivity
	status, latency, testErr := testLLMConnectivity(provider, apiKey)
	resp := &pb.TestAPIKeyResponse{
		Status:    status,
		LatencyMs: latency,
	}
	if testErr != nil {
		resp.Error = testErr.Error()
	}
	return connect.NewResponse(resp), nil
}

// getAIUsageStats queries the database for the authenticated user.
func (s *Service) getAIUsageStats(ctx context.Context, userID string) (*pb.AIUsageStats, error) {
	stats := &pb.AIUsageStats{}

	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(tokens_in + tokens_out), 0) FROM ai_usage_logs
		 WHERE user_id=$1 AND created_at::date = CURRENT_DATE`, userID).Scan(&stats.TodayTokens)
	if err != nil {
		return nil, fmt.Errorf("today tokens: %w", err)
	}

	err = s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(tokens_in + tokens_out), 0), COALESCE(SUM(cost_cents), 0)
		 FROM ai_usage_logs WHERE user_id=$1
		 AND date_trunc('month', created_at) = date_trunc('month', CURRENT_DATE)`,
		userID).Scan(&stats.MonthTokens, &stats.MonthCostCents)
	if err != nil {
		return nil, fmt.Errorf("month stats: %w", err)
	}

	err = s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(quota_limit_cents), 0) FROM user_api_keys WHERE user_id=$1`,
		userID).Scan(&stats.QuotaLimitCents)
	if err != nil {
		return nil, fmt.Errorf("quota limit: %w", err)
	}

	return stats, nil
}

// testLLMConnectivity makes a lightweight API call to verify the key works.
func testLLMConnectivity(provider, apiKey string) (status string, latencyMs int32, err error) {
	endpoint := ""
	switch strings.ToLower(provider) {
	case "openai":
		endpoint = "https://api.openai.com/v1/models"
	case "anthropic":
		endpoint = "https://api.anthropic.com/v1/messages"
	default:
		return "unknown_provider", 0, fmt.Errorf("不支持的 provider: %s", provider)
	}

	start := time.Now()
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "error", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	latencyMs = int32(time.Since(start).Milliseconds())
	if err != nil {
		return "down", latencyMs, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return "connected", latencyMs, nil
	}
	return "auth_failed", latencyMs, fmt.Errorf("HTTP %d", resp.StatusCode)
}
