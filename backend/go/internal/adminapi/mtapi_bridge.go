// Package adminapi — wrapper functions for admin-only MT API calls used during
// broker setup and connection testing. These are not routed through mthub
// because they operate before any session exists.
package adminapi

import (
	"context"
	"time"

	"github.com/alfq/backend/go/internal/common/config"
	mt "github.com/alfq/backend/go/internal/mdgateway/adapter/mtapi"
)

// brokerTestConnect tests connectivity to a broker and returns account metadata.
func brokerTestConnect(ctx context.Context, gw config.GatewayConfig, platform, login, password, host string) (*mt.AccountInfo, error) {
	return mt.TestConnect(ctx, gw, platform, login, password, host)
}

// brokerSearchOnline searches the gateway for matching brokers.
func brokerSearchOnline(ctx context.Context, gw config.GatewayConfig, platform, keyword string) ([]mt.BrokerMatch, error) {
	return mt.SearchBrokersOnline(ctx, gw, platform, keyword)
}

// brokerTestConnectWithTimeout wraps brokerTestConnect with a configurable timeout.
func brokerTestConnectWithTimeout(ctx context.Context, gw config.GatewayConfig, platform, login, password, host string, timeout time.Duration) (*mt.AccountInfo, error) {
	connectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return brokerTestConnect(connectCtx, gw, platform, login, password, host)
}
