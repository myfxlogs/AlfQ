// Package mdgateway — bar aggregator: tick → OHLCV bars.
package mdgateway

import (
	"sync"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Bar periods in milliseconds.
var periods = []int64{
	1 * 60 * 1000,       // 1m
	5 * 60 * 1000,       // 5m
	15 * 60 * 1000,      // 15m
	60 * 60 * 1000,      // 1h
	4 * 60 * 60 * 1000,  // 4h
	24 * 60 * 60 * 1000, // 1d
}

// periodName maps period ms to string.
var periodNames = map[int64]string{
	1 * 60 * 1000:        "1m",
	5 * 60 * 1000:        "5m",
	15 * 60 * 1000:       "15m",
	60 * 60 * 1000:       "1h",
	4 * 60 * 60 * 1000:   "4h",
	24 * 60 * 60 * 1000:  "1d",
}

// Bar is a completed OHLCV bar.
type Bar struct {
	TenantID       string
	Broker         string
	SymbolRaw      string
	Canonical      string
	Period         string
	OpenTsUnixMs   int64
	CloseTsUnixMs  int64
	Open           float64
	High           float64
	Low            float64
	Close          float64
	Volume         float64
	TickCount      uint32
}

// openBar is an in-progress bar.
type openBar struct {
	bar      Bar
	startMs  int64
	endMs    int64
}

// Aggregator accumulates ticks into bars.
type Aggregator struct {
	mu   sync.Mutex
	bars map[string]*openBar // key: "broker:symbol:period"
}

// NewAggregator creates a bar aggregator.
func NewAggregator() *Aggregator {
	return &Aggregator{
		bars: make(map[string]*openBar),
	}
}

// OnBar is called when a bar is completed.
type OnBar func(bar Bar)

// AddTick processes a tick and returns completed bars via onBar callback.
func (a *Aggregator) AddTick(tick *pb.Tick, onBar OnBar) {
	ts := tick.TsUnixMs
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	if tick.GetBid().GetValue() == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	broker := tick.Broker
	symbol := tick.Symbol

	for _, period := range periods {
		bucket := (ts / period) * period
		key := broker + ":" + symbol + ":" + periodNames[period]

		ob, ok := a.bars[key]
		if !ok || ob.startMs != bucket {
			// Flush old bar if exists
			if ok && ob.bar.TickCount > 0 {
				onBar(ob.bar)
			}
			// Start new bar
			ob = &openBar{
				startMs: bucket,
				endMs:   bucket + period - 1,
				bar: Bar{
					TenantID:      tick.TenantId,
					Broker:        broker,
					SymbolRaw:     symbol,
					Canonical:     tick.Canonical,
					Period:        periodNames[period],
					OpenTsUnixMs:  bucket,
					CloseTsUnixMs: bucket + period - 1,
				},
			}
			a.bars[key] = ob
		}

		bid, _ := parseFloat(tick.GetBid().GetValue())
		if bid == 0 {
			continue
		}
		ob.bar.Close = bid
		if ob.bar.TickCount == 0 {
			ob.bar.Open = bid
			ob.bar.High = bid
			ob.bar.Low = bid
		} else {
			if bid > ob.bar.High {
				ob.bar.High = bid
			}
			if bid < ob.bar.Low {
				ob.bar.Low = bid
			}
		}
		ob.bar.Volume += tick.BidVolume + tick.AskVolume
		ob.bar.TickCount++
	}
}

// FlushAll flushes all open bars via onBar callback.
func (a *Aggregator) FlushAll(onBar OnBar) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, ob := range a.bars {
		if ob.bar.TickCount > 0 {
			onBar(ob.bar)
		}
	}
	a.bars = make(map[string]*openBar)
}

// Size returns the number of open bars.
func (a *Aggregator) Size() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.bars)
}

func parseFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	val, err := fastFloat(s)
	return val, err == nil
}

// fastFloat parses a string to float64 without strconv.
func fastFloat(s string) (float64, error) {
	var intPart, fracPart float64
	var fracDiv float64 = 1
	inFrac := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			inFrac = true
			continue
		}
		if c < '0' || c > '9' {
			continue
		}
		if inFrac {
			fracPart = fracPart*10 + float64(c-'0')
			fracDiv *= 10
		} else {
			intPart = intPart*10 + float64(c-'0')
		}
	}
	if fracDiv > 1 {
		return intPart + fracPart/fracDiv, nil
	}
	return intPart, nil
}
