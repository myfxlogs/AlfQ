// Package mdgateway — market data normalizer.
package mdgateway

import (
	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Normalizer converts broker-specific quote types to alfq.v1.Tick.
// Placeholder — actual conversion logic lives in each broker adapter.
type Normalizer struct{}

// Tick creates a Tick with common fields filled.
func (n *Normalizer) Tick(tenantID, broker, symbol string, tsMs int64, bid, ask string) *pb.Tick {
	return &pb.Tick{
		TenantId:      tenantID,
		Broker:        broker,
		Symbol:        symbol,
		TsUnixMs:      tsMs,
		Bid:           &pb.Money{Value: bid},
		Ask:           &pb.Money{Value: ask},
	}
}
