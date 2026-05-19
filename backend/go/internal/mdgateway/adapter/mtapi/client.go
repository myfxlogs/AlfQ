// Package mtapi — MT4/MT5 gRPC client: broker search + account connection.
package mtapi

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/alfq/backend/go/internal/common/config"
	mt5pb "github.com/alfq/backend/go/gen/mt5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	Servers []string
}

// ── Online broker search ──

// SearchBrokersOnline queries an MT gRPC gateway for broker companies.
func SearchBrokersOnline(ctx context.Context, gw config.GatewayConfig, mtType, company string) ([]BrokerMatch, error) {
	conn, err := dial(ctx, gw)
	if err != nil {
		return nil, fmt.Errorf("mtapi: dial %s gateway: %w", mtType, err)
	}
	defer conn.Close()

	switch strings.ToUpper(mtType) {
	case "MT5":
		return searchMT5(ctx, conn, company)
	case "MT4":
		return searchMT4(ctx, conn, company)
	default:
		return nil, fmt.Errorf("mtapi: unsupported platform %q", mtType)
	}
}

func searchMT5(ctx context.Context, conn *grpc.ClientConn, company string) ([]BrokerMatch, error) {
	client := mt5pb.NewServiceClient(conn)
	resp, err := client.Search(ctx, &mt5pb.SearchRequest{Company: company})
	if err != nil {
		return nil, fmt.Errorf("mtapi: mt5 search: %w", err)
	}
	var matches []BrokerMatch
	for _, c := range resp.GetResult() {
		var servers []string
		for _, r := range c.GetResults() {
			servers = append(servers, r.GetAccess()...)
		}
		matches = append(matches, BrokerMatch{Company: c.GetCompanyName(), Servers: servers})
	}
	return matches, nil
}

func searchMT4(ctx context.Context, conn *grpc.ClientConn, company string) ([]BrokerMatch, error) {
	// MT4 gateway uses same Service/Search pattern
	input := map[string]interface{}{"company": company}
	output := make(map[string]interface{})
	if err := conn.Invoke(ctx, "/mt4grpc.Service/Search", input, output); err != nil {
		return nil, fmt.Errorf("mtapi: mt4 search: %w", err)
	}
	result, _ := output["result"].([]interface{})
	var matches []BrokerMatch
	for _, r := range result {
		c, _ := r.(map[string]interface{})
		companyName, _ := c["companyName"].(string)
		results, _ := c["results"].([]interface{})
		var servers []string
		for _, rr := range results {
			m, _ := rr.(map[string]interface{})
			access, _ := m["access"].([]interface{})
			for _, a := range access {
				if s, ok := a.(string); ok {
					servers = append(servers, s)
				}
			}
		}
		matches = append(matches, BrokerMatch{Company: companyName, Servers: servers})
	}
	return matches, nil
}

// ── Account connection ──

// TestConnect attempts to connect via gateway and returns account info.
func TestConnect(ctx context.Context, gw config.GatewayConfig, mtType, login, password, brokerHostPort string) (*AccountInfo, error) {
	conn, err := dial(ctx, gw)
	if err != nil {
		return nil, fmt.Errorf("mtapi: dial gateway: %w", err)
	}
	defer conn.Close()

	host, port := splitHostPort(brokerHostPort, "443")

	switch strings.ToUpper(mtType) {
	case "MT5":
		return connectMT5(ctx, conn, login, password, host, parsePort(port))
	case "MT4":
		return connectMT4(ctx, conn, login, password, host, port)
	default:
		return nil, fmt.Errorf("mtapi: unsupported platform %q", mtType)
	}
}

func connectMT5(ctx context.Context, conn *grpc.ClientConn, login, password, host string, port int32) (*AccountInfo, error) {
	connClient := mt5pb.NewConnectionClient(conn)
	resp, err := connClient.Connect(ctx, &mt5pb.ConnectRequest{
		User:     parseUint(login),
		Password: password,
		Host:     host,
		Port:     port,
	})
	if err != nil {
		return nil, fmt.Errorf("mtapi: mt5 connect: %w", err)
	}
	if resp.GetError() != nil && resp.GetError().GetMessage() != "" {
		return nil, fmt.Errorf("mtapi: mt5 error: %s", resp.GetError().GetMessage())
	}
	return getAccountSummary(ctx, conn, "/mt5grpc.Connection/AccountSummary")
}

func connectMT4(ctx context.Context, conn *grpc.ClientConn, login, password, host, port string) (*AccountInfo, error) {
	md := map[string]interface{}{
		"user": login, "password": password, "host": host, "port": port,
	}
	output := make(map[string]interface{})
	if err := conn.Invoke(ctx, "/mt4grpc.Connection/Connect", md, output); err != nil {
		return nil, fmt.Errorf("mtapi: mt4 connect: %w", err)
	}
	return getAccountSummary(ctx, conn, "/mt4grpc.Connection/AccountSummary")
}

func getAccountSummary(ctx context.Context, conn *grpc.ClientConn, method string) (*AccountInfo, error) {
	summary := make(map[string]interface{})
	conn.Invoke(ctx, method, map[string]interface{}{}, summary)
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

// ── Builtin fallback ──

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

// ── internal ──

func dial(ctx context.Context, gw config.GatewayConfig) (*grpc.ClientConn, error) {
	dialOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTimeout(gw.Timeout)}
	if gw.UseTLS {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	return grpc.DialContext(ctx, gw.Addr, dialOpts...)
}

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

func parsePort(s string) int32 { n := parseUint(s); if n == 0 { return 443 }; return int32(n) }

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok { return v }
	return 0
}
func getString(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}
