// Package mt5 包装 mtapi MT5 gRPC 客户端。
//
// 远端服务器：mt5grpc3.mtapi.io:443（TLS）。
// 与 MT4 完全独立，不可共享 proto 类型；详见 docs/14 §3.4。
package mt5

import (
	"context"
	"crypto/tls"
	"fmt"

	mt5pb "github.com/alfq/backend/go/gen/mt5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// DefaultEndpoint 是 mtapi 官方 MT5 远端 gRPC 入口。
const DefaultEndpoint = "mt5grpc3.mtapi.io:443"

// Client 持有 MT5 各 service 客户端的句柄。
type Client struct {
	cc *grpc.ClientConn

	Connection    mt5pb.ConnectionClient
	MT5           mt5pb.MT5Client
	QuoteHistory  mt5pb.QuoteHistoryClient
	TickHistory   mt5pb.TickHistoryClient
	Trading       mt5pb.TradingClient
	Service       mt5pb.ServiceClient
	Subscriptions mt5pb.SubscriptionsClient
	Streams       mt5pb.StreamsClient
}

// Dial 连接到 MT5 mtapi gRPC 服务器（TLS）。
func Dial(ctx context.Context, endpoint string, opts ...grpc.DialOption) (*Client, error) {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	dialOpts := append([]grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	}, opts...)
	cc, err := grpc.NewClient(endpoint, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("mt5: dial %s: %w", endpoint, err)
	}
	return &Client{
		cc:            cc,
		Connection:    mt5pb.NewConnectionClient(cc),
		MT5:           mt5pb.NewMT5Client(cc),
		QuoteHistory:  mt5pb.NewQuoteHistoryClient(cc),
		TickHistory:   mt5pb.NewTickHistoryClient(cc),
		Trading:       mt5pb.NewTradingClient(cc),
		Service:       mt5pb.NewServiceClient(cc),
		Subscriptions: mt5pb.NewSubscriptionsClient(cc),
		Streams:       mt5pb.NewStreamsClient(cc),
	}, nil
}

// Close 关闭底层 gRPC 连接。
func (c *Client) Close() error {
	if c.cc != nil {
		return c.cc.Close()
	}
	return nil
}
