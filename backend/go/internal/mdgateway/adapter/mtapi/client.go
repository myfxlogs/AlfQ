// Package mtapi — MT4/MT5 gRPC client: broker search + account connection.
package mtapi

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	mt5pb "github.com/alfq/backend/go/gen/mt5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AccountInfo returned from MT connection test.
type AccountInfo struct {
	Balance     float64
	Equity      float64
	Margin      float64
	FreeMargin  float64
	MarginLevel float64
	Profit      float64
	Currency    string
	Leverage    int32
}

// BrokerMatch from online broker search.
type BrokerMatch struct {
	Company string
	Servers []string // e.g. ["mt5-demo.roboforex.com:443"]
}

// MT5GatewayAddr returns the configured MT5 gRPC gateway address.
func MT5GatewayAddr() string {
	if addr := os.Getenv("MT5_GATEWAY_ADDR"); addr != "" {
		return addr
	}
	return "mt5gateway:443" // default in docker compose
}

// SearchBrokersOnline queries the MT5 gRPC gateway for broker companies.
func SearchBrokersOnline(ctx context.Context, gatewayAddr, company string) ([]BrokerMatch, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, gatewayAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("mtapi: dial gateway %s: %w", gatewayAddr, err)
	}
	defer conn.Close()

	client := mt5pb.NewServiceClient(conn)
	resp, err := client.Search(ctx, &mt5pb.SearchRequest{Company: company})
	if err != nil {
		return nil, fmt.Errorf("mtapi: search: %w", err)
	}

	var matches []BrokerMatch
	for _, c := range resp.GetResult() {
		var servers []string
		for _, r := range c.GetResults() {
			servers = append(servers, r.GetAccess()...)
		}
		matches = append(matches, BrokerMatch{
			Company: c.GetCompanyName(),
			Servers: servers,
		})
	}
	return matches, nil
}

// TestConnectMT5 connects to an MT5 account via the gateway and returns account info.
// gatewayAddr: the MT5 gRPC gateway address (e.g. "mt5gateway:443")
// brokerHost/port: the actual broker server
func TestConnectMT5(ctx context.Context, gatewayAddr, login, password, brokerHost string, brokerPort int32) (*AccountInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, gatewayAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("mtapi: dial gateway: %w", err)
	}
	defer conn.Close()

	connClient := mt5pb.NewConnectionClient(conn)
	connectResp, err := connClient.Connect(ctx, &mt5pb.ConnectRequest{
		User:     parseUint(login),
		Password: password,
		Host:     brokerHost,
		Port:     brokerPort,
	})
	if err != nil {
		return nil, fmt.Errorf("mtapi: connect: %w", err)
	}
	if connectResp.GetError() != nil && connectResp.GetError().GetMessage() != "" {
		return nil, fmt.Errorf("mtapi: mt5 error: %s", connectResp.GetError().GetMessage())
	}

	// Get account summary via raw gRPC (generated client doesn't expose it)
	summary := make(map[string]interface{})
	conn.Invoke(ctx, "/mt5grpc.Connection/AccountSummary", map[string]interface{}{}, summary)
	result, _ := summary["result"].(map[string]interface{})
	if result == nil {
		return &AccountInfo{}, nil
	}
	return &AccountInfo{
		Balance:     getFloat(result, "balance"),
		Equity:      getFloat(result, "equity"),
		Margin:      getFloat(result, "margin"),
		FreeMargin:  getFloat(result, "freeMargin"),
		MarginLevel: getFloat(result, "marginLevel"),
		Profit:      getFloat(result, "profit"),
		Currency:    getString(result, "currency"),
		Leverage:    int32(getFloat(result, "leverage")),
	}, nil
}

// TestConnect attempts MT5 connection via gateway, falls back to direct gRPC for MT4.
func TestConnect(ctx context.Context, mtType, login, password, hostPort string) (*AccountInfo, error) {
	gateway := MT5GatewayAddr()
	if strings.ToUpper(mtType) == "MT5" {
		host, port := splitHostPort(hostPort, "443")
		return TestConnectMT5(ctx, gateway, login, password, host, parsePort(port))
	}
	// MT4: direct gRPC (fallback)
	return testConnectDirect(ctx, login, password, hostPort)
}

// BuiltinBrokers returns a hardcoded list of well-known brokers as fallback.
func BuiltinBrokers() []BrokerMatch {
	return []BrokerMatch{
		{Company: "RoboForex", Servers: []string{"mt4-demo.roboforex.com:443", "mt5-demo.roboforex.com:443"}},
		{Company: "IC Markets", Servers: []string{"mt4-demo.icmarkets.com:443", "mt5-demo.icmarkets.com:443"}},
		{Company: "XM", Servers: []string{"mt4-demo.xm.com:443", "mt5-demo.xm.com:443"}},
		{Company: "Exness", Servers: []string{"mt4-demo.exness.com:443", "mt5-demo.exness.com:443"}},
		{Company: "Pepperstone", Servers: []string{"mt4-demo.pepperstone.com:443", "mt5-demo.pepperstone.com:443"}},
		{Company: "Tickmill", Servers: []string{"mt4-demo.tickmill.com:443", "mt5-demo.tickmill.com:443"}},
		{Company: "FP Markets", Servers: []string{"mt4-demo.fpmarkets.com:443", "mt5-demo.fpmarkets.com:443"}},
		{Company: "FBS", Servers: []string{"mt4-demo.fbs.com:443", "mt5-demo.fbs.com:443"}},
	}
}

// ── helpers ──

func splitHostPort(hostPort, defaultPort string) (string, string) {
	parts := strings.Split(hostPort, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return hostPort, defaultPort
}

func parseUint(s string) uint64 {
	var n uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint64(c-'0')
		}
	}
	return n
}

func parsePort(s string) int32 {
	n := parseUint(s)
	if n == 0 {
		return 443
	}
	return int32(n)
}

// ── low-level direct connect (MT4 / fallback) ──

func testConnectDirect(ctx context.Context, login, password, hostPort string) (*AccountInfo, error) {
	conn, err := grpc.DialContext(ctx, hostPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("mtapi: dial %s: %w", hostPort, err)
	}
	defer conn.Close()

	host, port := splitHostPort(hostPort, "443")
	md := map[string]interface{}{
		"user": login, "password": password, "host": host, "port": port,
	}
	output := make(map[string]interface{})
	if err := conn.Invoke(ctx, "/mt4grpc.Connection/Connect", md, output); err != nil {
		return nil, fmt.Errorf("mtapi: connect: %w", err)
	}

	summary := make(map[string]interface{})
	conn.Invoke(ctx, "/mt4grpc.Connection/AccountSummary", map[string]interface{}{}, summary)
	return &AccountInfo{
		Balance:     getFloat(summary, "balance"),
		Equity:      getFloat(summary, "equity"),
		Margin:      getFloat(summary, "margin"),
		FreeMargin:  getFloat(summary, "freeMargin"),
		MarginLevel: getFloat(summary, "marginLevel"),
		Profit:      getFloat(summary, "profit"),
		Currency:    getString(summary, "currency"),
		Leverage:    int32(getFloat(summary, "leverage")),
	}, nil
}

func getFloat(m map[string]interface{}, key string) float64 {
	v, _ := m[key]
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	}
	return 0
}

func getString(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}
