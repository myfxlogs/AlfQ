// Package marketdataview provides a unified bar query interface backed by ClickHouse (RS01).
// Implements MarketDataViewService: Bars (stream) and LatestBar.
// Used by factorsvc and quantengine instead of direct NATS bar consumption.
package marketdataview

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"go.uber.org/zap"
)

// CHView queries ClickHouse for historical OHLCV bars.
type CHView struct {
	ch  *pgxpool.Pool // ClickHouse via pgx protocol
	log *zap.Logger
}

// NewCHView creates a ClickHouse-backed bar query view.
func NewCHView(ch *pgxpool.Pool, log *zap.Logger) *CHView {
	return &CHView{ch: ch, log: log}
}

// Bars streams bars matching the query. If CH is unavailable, returns empty.
func (v *CHView) Bars(ctx context.Context, req *pb.BarQuery, stream *connect.ServerStream[pb.BarView]) error {
	if v.ch == nil {
		return nil
	}

	rows, err := v.ch.Query(ctx, `
		SELECT tenant_id, symbol, period, ts_unix_ms, open, high, low, close, volume
		FROM md_bars
		WHERE tenant_id = $1 AND symbol = $2 AND period = $3
		  AND ts_unix_ms >= $4 AND ts_unix_ms < $5
		ORDER BY ts_unix_ms ASC
		LIMIT $6
	`, req.TenantId, req.Symbol, req.Period, req.FromMs, req.ToMs, req.Limit)
	if err != nil {
		v.log.Warn("ch bars query failed", zap.Error(err))
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		bv := &pb.BarView{}
		if err := rows.Scan(&bv.TenantId, &bv.Symbol, &bv.Period, &bv.TsUnixMs,
			&bv.Open, &bv.High, &bv.Low, &bv.Close, &bv.Volume); err != nil {
			v.log.Warn("ch bar scan failed", zap.Error(err))
			continue
		}
		if err := stream.Send(bv); err != nil {
			return err
		}
	}
	return rows.Err()
}

// LatestBar returns the most recent bar for a symbol/period pair.
func (v *CHView) LatestBar(ctx context.Context, req *pb.BarLookup) (*pb.BarView, error) {
	if v.ch == nil {
		return nil, fmt.Errorf("ch not available")
	}

	bv := &pb.BarView{}
	err := v.ch.QueryRow(ctx, `
		SELECT tenant_id, symbol, period, ts_unix_ms, open, high, low, close, volume
		FROM md_bars
		WHERE tenant_id = $1 AND symbol = $2 AND period = $3
		ORDER BY ts_unix_ms DESC
		LIMIT 1
	`, req.TenantId, req.Symbol, req.Period).Scan(
		&bv.TenantId, &bv.Symbol, &bv.Period, &bv.TsUnixMs,
		&bv.Open, &bv.High, &bv.Low, &bv.Close, &bv.Volume,
	)
	if err != nil {
		return nil, fmt.Errorf("latest bar: %w", err)
	}
	return bv, nil
}

// MemoryView is an in-memory fixture for testing.
type MemoryView struct {
	bars map[string][]*pb.BarView // key: "tenant_id/symbol/period"
}

// NewMemoryView creates an in-memory bar store for testing.
func NewMemoryView() *MemoryView {
	return &MemoryView{bars: make(map[string][]*pb.BarView)}
}

// Add injects bars into the memory view.
func (m *MemoryView) Add(bars ...*pb.BarView) {
	for _, b := range bars {
		key := b.TenantId + "/" + b.Symbol + "/" + b.Period
		m.bars[key] = append(m.bars[key], b)
	}
}

// LatestBar returns the most recent bar from memory.
func (m *MemoryView) LatestBar(ctx context.Context, req *pb.BarLookup) (*pb.BarView, error) {
	key := req.TenantId + "/" + req.Symbol + "/" + req.Period
	list := m.bars[key]
	if len(list) == 0 {
		return nil, fmt.Errorf("no bars for %s", key)
	}
	return list[len(list)-1], nil
}
