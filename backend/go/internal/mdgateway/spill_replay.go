// Package mdgateway — spill file replay on startup.
package mdgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// SpillReplay reads JSONL spill files and writes them to ClickHouse.
// Files are moved to a "processed" subdirectory on success.
type SpillReplay struct {
	dir    string
	log    *zap.Logger
	chConn *CHConn
}

// NewSpillReplay creates a spill replay worker.
func NewSpillReplay(dir string, chConn *CHConn, log *zap.Logger) *SpillReplay {
	return &SpillReplay{dir: dir, log: log, chConn: chConn}
}

// Replay processes all .jsonl files in the spill directory.
func (s *SpillReplay) Replay(ctx context.Context) (int, error) {
	if s.dir == "" {
		return 0, nil
	}
	if err := os.MkdirAll(s.dir+"/processed", 0o755); err != nil {
		return 0, fmt.Errorf("spill: mkdir processed: %w", err)
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("spill: readdir: %w", err)
	}

	total := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		count, err := s.replayFile(ctx, path)
		if err != nil {
			s.log.Warn("spill: replay failed",
				zap.String("file", e.Name()),
				zap.Error(err),
			)
			continue
		}
		total += count

		// Move to processed
		dest := filepath.Join(s.dir, "processed", e.Name())
		if err := os.Rename(path, dest); err != nil {
			s.log.Warn("spill: rename failed",
				zap.String("file", e.Name()),
				zap.Error(err),
			)
		}
	}

	if total > 0 {
		s.log.Info("spill: replay complete", zap.Int("rows", total))
	}
	return total, nil
}

func (s *SpillReplay) replayFile(ctx context.Context, path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	conn, err := s.chConn.Conn(ctx)
	if err != nil {
		return 0, err
	}

	insCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	batch, err := conn.PrepareBatch(insCtx, "INSERT INTO md_ticks")
	if err != nil {
		return 0, err
	}

	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec struct {
			TenantID      string  `json:"tenant_id"`
			Broker        string  `json:"broker"`
			Symbol        string  `json:"symbol"`
			TsUnixMs      int64   `json:"ts_unix_ms"`
			ArrivedUnixMs int64   `json:"arrived_unix_ms"`
			Bid           float64 `json:"bid"`
			Ask           float64 `json:"ask"`
			BidVolume     float64 `json:"bid_volume"`
			AskVolume     float64 `json:"ask_volume"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			s.log.Warn("spill: parse line", zap.Error(err))
			continue
		}
		// CH driver v2.46+ natively converts float64 → Decimal(18,6).
		_ = batch.Append(
			rec.TenantID, rec.Broker, rec.Symbol, rec.Symbol,
			uint64(rec.TsUnixMs), uint64(rec.ArrivedUnixMs),
			rec.Bid, rec.Ask, rec.BidVolume, rec.AskVolume,
		)
		count++
	}
	if count > 0 {
		if err := batch.Send(); err != nil {
			return 0, err
		}
	}
	return count, nil
}
