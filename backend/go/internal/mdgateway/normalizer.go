// Package mdgateway — market data normalizer.
package mdgateway

import (
	"sync"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/symbolsync"
)

// CanonicalResolver resolves (broker, symbol_raw) → canonical name.
// Implementations may use broker_symbols PG table, in-memory cache,
// or fall back to algorithmic canonicalize().
type CanonicalResolver interface {
	Resolve(brokerID, symbolRaw string) string
}

// mapResolver is a simple in-memory map backed by symbolsync.Canonicalize() fallback.
type mapResolver struct {
	mu    sync.RWMutex
	cache map[string]string // key: "broker:symbolRaw"
}

// NewMapResolver creates a resolver backed by an in-memory LRU-like map.
// Misses fall back to algorithmic canonicalize (suffix stripping).
func NewMapResolver() CanonicalResolver {
	return &mapResolver{
		cache: make(map[string]string),
	}
}

func (r *mapResolver) Resolve(brokerID, symbolRaw string) string {
	key := brokerID + ":" + symbolRaw
	r.mu.RLock()
	if c, ok := r.cache[key]; ok {
		r.mu.RUnlock()
		return c
	}
	r.mu.RUnlock()

	// Fallback: algorithmic canonicalize
	canon := symbolsync.Canonicalize(symbolRaw)

	r.mu.Lock()
	r.cache[key] = canon
	r.mu.Unlock()
	return canon
}

// Load pre-populates the resolver cache from a broker_symbols result set.
func (r *mapResolver) Load(brokerID, symbolRaw, canonical string) {
	key := brokerID + ":" + symbolRaw
	r.mu.Lock()
	r.cache[key] = canonical
	r.mu.Unlock()
}

// Normalizer converts broker-specific quote types to alfq.v1.Tick.
type Normalizer struct {
	resolver CanonicalResolver
}

// NewNormalizer creates a Normalizer with an optional canonical resolver.
// If resolver is nil, canonical defaults to the raw symbol.
func NewNormalizer(resolver CanonicalResolver) *Normalizer {
	return &Normalizer{resolver: resolver}
}

// Tick creates a Tick with common fields filled, including canonical name.
func (n *Normalizer) Tick(tenantID, broker, symbol string, tsMs int64, bid, ask string) *pb.Tick {
	canon := symbol
	if n.resolver != nil {
		canon = n.resolver.Resolve(broker, symbol)
	}
	return &pb.Tick{
		TenantId:  tenantID,
		Broker:    broker,
		Symbol:    symbol,
		Canonical: canon,
		TsUnixMs:  tsMs,
		Bid:       &pb.Money{Value: bid},
		Ask:       &pb.Money{Value: ask},
	}
}
