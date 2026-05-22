// Command factor-golden generates Go DSL golden values for RS01 parity tests.
//
// Usage: go run ./cmd/factor-golden
//
// Outputs:
//   research/tests/fixtures/golden_bars.json     — 100 OHLC bars (fixture)
//   research/tests/fixtures/go_golden_values.json — SMA20/EMA60/RSI14 golden values
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/alfq/backend/go/internal/factor/dsl"
)

// OHLCBar is a fixture bar with OHLCV data.
type OHLCBar struct {
	TsUnixMs int64   `json:"ts_unix_ms"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	Volume   float64 `json:"volume"`
}

// GoldenOutput holds the Go-computed factor values.
// Uses *float64 so NaN values serialize as JSON null.
type GoldenOutput struct {
	SMA20 []*float64 `json:"sma20"`
	EMA60 []*float64 `json:"ema60"`
	RSI14 []*float64 `json:"rsi14"`
}

func main() {
	projectRoot := findProjectRoot()
	barsPath := filepath.Join(projectRoot, "research", "tests", "fixtures", "golden_bars.json")
	goldenPath := filepath.Join(projectRoot, "research", "tests", "fixtures", "go_golden_values.json")

	// Generate deterministic bars
	bars := generateBars(100)
	writeJSON(barsPath, bars)
	fmt.Printf("wrote %d bars → %s\n", len(bars), barsPath)

	// Compute Go DSL golden values
	golden := computeGolden(bars)
	writeJSON(goldenPath, golden)
	fmt.Printf("wrote golden values → %s\n", goldenPath)
}

func findProjectRoot() string {
	// Walk up from cwd until we find go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// in backend/go — go up two levels
			return filepath.Join(dir, "..", "..")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func generateBars(n int) []OHLCBar {
	bars := make([]OHLCBar, n)
	price := 1.1000
	for i := 0; i < n; i++ {
		// Deterministic sine-wave + linear drift pattern
		price += 0.0001*math.Sin(float64(i)*0.15) + 0.00005
		price = math.Max(price, 0.5)

		openP := price
		closeP := price + 0.00005*float64((i*7)%20-10)/10.0
		highP := math.Max(openP, closeP) + math.Abs(0.00008*math.Sin(float64(i)*0.3))
		lowP := math.Min(openP, closeP) - math.Abs(0.00008*math.Cos(float64(i)*0.3))

		bars[i] = OHLCBar{
			TsUnixMs: 1_700_000_000_000 + int64(i)*300_000,
			Open:     round8(openP),
			High:     round8(highP),
			Low:      round8(lowP),
			Close:    round8(closeP),
			Volume:   100.0 + float64(i%3)*50.0,
		}
	}
	return bars
}

func round8(v float64) float64 {
	return math.Round(v*1e8) / 1e8
}

func computeGolden(bars []OHLCBar) GoldenOutput {
	sma := dsl.NewSMA(20)
	ema := dsl.NewEMA(60)
	rsi := dsl.NewRSI(14)

	smaVals := make([]*float64, len(bars))
	emaVals := make([]*float64, len(bars))
	rsiVals := make([]*float64, len(bars))

	for i, b := range bars {
		smaVals[i] = nanOrNull(sma.Eval(b.Close))
		emaVals[i] = nanOrNull(ema.Eval(b.Close))
		rsiVals[i] = nanOrNull(rsi.Eval(b.Close))
	}

	return GoldenOutput{SMA20: smaVals, EMA60: emaVals, RSI14: rsiVals}
}

// nanOrNull returns nil for NaN, value otherwise (JSON-safe).
func nanOrNull(v float64) *float64 {
	if math.IsNaN(v) {
		return nil
	}
	return &v
}

func writeJSON(path string, v any) {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
}
