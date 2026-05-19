// Package mtapi — lightweight MT4/MT5 gRPC connection tester.
// Uses generated gRPC clients from gprc/ proto definitions.
package mtapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AccountInfo returned from MT connection test.
type AccountInfo struct {
	Balance      float64
	Equity       float64
	Margin       float64
	FreeMargin   float64
	MarginLevel  float64
	Profit       float64
	Currency     string
	Leverage     int32
	AccountType  string
}

// TestConnect attempts to connect to an MT4/MT5 account and returns account info.
// hostPort: e.g. "mt4-demo.roboforex.com:443"
func TestConnect(ctx context.Context, mtType, login, password, hostPort string) (*AccountInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, hostPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("mtapi: dial %s: %w", hostPort, err)
	}
	defer conn.Close()

	switch strings.ToUpper(mtType) {
	case "MT4":
		return connectMT4(ctx, conn, login, password, hostPort)
	case "MT5":
		return connectMT5(ctx, conn, login, password, hostPort)
	default:
		return nil, fmt.Errorf("mtapi: unsupported mt_type %q", mtType)
	}
}

func connectMT4(ctx context.Context, conn *grpc.ClientConn, login, password, hostPort string) (*AccountInfo, error) {
	// Use the generated MT4 gRPC client.
	// Since MT4 proto uses mt4grpc.Connection service with Connect RPC,
	// we do a raw gRPC call.
	var info AccountInfo

	// MT4 Connect RPC: mt4grpc.Connection/Connect
	// Input: {"user":"...","password":"...","host":"...","port":"443"}
	md := createMT4Input(login, password, hostPort)
	output := make(map[string]interface{})
	err := conn.Invoke(ctx, "/mt4grpc.Connection/Connect", md, output)
	if err != nil {
		// MT connection failure is expected for invalid creds
		return nil, fmt.Errorf("mtapi: mt4 connect: %w", err)
	}

	// Try AccountSummary RPC
	summary := make(map[string]interface{})
	if err := conn.Invoke(ctx, "/mt4grpc.Connection/AccountSummary", map[string]interface{}{}, summary); err == nil {
		info.Balance = getFloat(summary, "balance")
		info.Equity = getFloat(summary, "equity")
		info.Margin = getFloat(summary, "margin")
		info.FreeMargin = getFloat(summary, "freeMargin")
		info.MarginLevel = getFloat(summary, "marginLevel")
		info.Profit = getFloat(summary, "profit")
		info.Currency = getString(summary, "currency")
		info.Leverage = int32(getFloat(summary, "leverage"))
	}
	return &info, nil
}

func connectMT5(ctx context.Context, conn *grpc.ClientConn, login, password, hostPort string) (*AccountInfo, error) {
	var info AccountInfo
	md := createMT5Input(login, password, hostPort)
	output := make(map[string]interface{})
	err := conn.Invoke(ctx, "/mt5grpc.Connection/Connect", md, output)
	if err != nil {
		return nil, fmt.Errorf("mtapi: mt5 connect: %w", err)
	}
	summary := make(map[string]interface{})
	if err := conn.Invoke(ctx, "/mt5grpc.Connection/AccountSummary", map[string]interface{}{}, summary); err == nil {
		info.Balance = getFloat(summary, "balance")
		info.Equity = getFloat(summary, "equity")
		info.Margin = getFloat(summary, "margin")
		info.FreeMargin = getFloat(summary, "freeMargin")
		info.MarginLevel = getFloat(summary, "marginLevel")
		info.Profit = getFloat(summary, "profit")
		info.Currency = getString(summary, "currency")
		info.Leverage = int32(getFloat(summary, "leverage"))
	}
	return &info, nil
}

func createMT4Input(login, password, hostPort string) map[string]interface{} {
	host, port := splitHostPort(hostPort, "443")
	return map[string]interface{}{
		"user":     login,
		"password": password,
		"host":     host,
		"port":     port,
	}
}

func createMT5Input(login, password, hostPort string) map[string]interface{} {
	host, port := splitHostPort(hostPort, "443")
	return map[string]interface{}{
		"login":    login,
		"password": password,
		"host":     host,
		"port":     port,
	}
}

func splitHostPort(hostPort, defaultPort string) (string, string) {
	parts := strings.Split(hostPort, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return hostPort, defaultPort
}

func getFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
