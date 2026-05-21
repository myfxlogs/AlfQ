// Package mt4 包装 mtapi MT4 gRPC 客户端。
//
// 远端服务器：mt4grpc3.mtapi.io:443（TLS）。
// 与 MT5 完全独立，不可共享 proto 类型；详见 docs/14 §3.4。
package mt4

import (
	"context"
	"crypto/tls"
	"fmt"

	mt4pb "github.com/alfq/backend/go/gen/mt4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// DefaultEndpoint 是 mtapi 官方 MT4 远端 gRPC 入口。
const DefaultEndpoint = "mt4grpc3.mtapi.io:443"

// Client 持有 MT4 各 service 客户端的句柄。
type Client struct {
	cc *grpc.ClientConn

	Connection    mt4pb.ConnectionClient
	MT4           mt4pb.MT4Client
	Trading       mt4pb.TradingClient
	Service       mt4pb.ServiceClient
	Subscriptions mt4pb.SubscriptionsClient
	Streams       mt4pb.StreamsClient
}

// Conn returns the underlying gRPC connection.
func (c *Client) Conn() *grpc.ClientConn { return c.cc }

// Dial 连接到 MT4 mtapi gRPC 服务器（TLS）。
func Dial(ctx context.Context, endpoint string, opts ...grpc.DialOption) (*Client, error) {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	dialOpts := append([]grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	}, opts...)
	cc, err := grpc.NewClient(endpoint, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("mt4: dial %s: %w", endpoint, err)
	}
	return &Client{
		cc:            cc,
		Connection:    mt4pb.NewConnectionClient(cc),
		MT4:           mt4pb.NewMT4Client(cc),
		Trading:       mt4pb.NewTradingClient(cc),
		Service:       mt4pb.NewServiceClient(cc),
		Subscriptions: mt4pb.NewSubscriptionsClient(cc),
		Streams:       mt4pb.NewStreamsClient(cc),
	}, nil
}

// Close 关闭底层 gRPC 连接。
func (c *Client) Close() error {
	if c.cc != nil {
		return c.cc.Close()
	}
	return nil
}
