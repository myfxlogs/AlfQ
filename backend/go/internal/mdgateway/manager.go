// Package mdgateway implements the market data gateway service.
//
// Adapter layout per docs/25:
//
//	internal/mdgateway/adapter/mt4/   → raw mtapi gRPC client wrapper
//	internal/mdgateway/adapter/mt5/   → raw mtapi gRPC client wrapper
//
// This manager orchestrates connections, platform dispatch,
// and tick fan-out to publisher + ClickHouse writer.
package mdgateway

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"google.golang.org/grpc"
)

// TickHandler is called for each normalized Tick.
type TickHandler func(tick *pb.Tick)

// Gateway wraps a single broker connection with tick streaming.
type Gateway interface {
	Platform() string
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Subscribe(ctx context.Context, symbols []string, handler TickHandler) error
	HealthCheck(ctx context.Context) error
	// Conn returns the underlying gRPC connection (may be nil before Connect).
	Conn() *grpc.ClientConn
	// SessionID returns the MT session token (empty before Connect).
	SessionID() string
	// BrokerID returns the broker UUID for this connection.
	BrokerID() string
}

// AccountConfig holds connection parameters for a broker account.
type AccountConfig struct {
	Broker     string
	Platform   string // "mt4" or "mt5"
	Login      string
	Password   string
	Server     string
	Host       string
	Port       string
	MtapiToken string
	TenantID   string
}

// Config holds the md-gateway service configuration.
type Config struct {
	Accounts []AccountEntry
	Log      LogConfig
}

// AccountEntry maps a broker account from config.
type AccountEntry struct {
	TenantID string
	Broker   string
	Platform string
	Login    string
	Password string
	Server   string
	Host     string
	Port     string
	Symbols  []string
}

// LogConfig holds log settings.
type LogConfig struct {
	Level string
}

// Manager manages broker connections.
type Manager struct {
	mu         sync.Mutex
	gateways   map[string]Gateway
	normalizer *Normalizer
}

// NewEmptyManager creates a Manager with no gateways (populated dynamically).
func NewEmptyManager() *Manager {
	return &Manager{gateways: make(map[string]Gateway)}
}

// SetNormalizer sets the canonical resolver for future gateway connections.
func (m *Manager) SetNormalizer(n *Normalizer) {
	m.normalizer = n
}

// NewManager creates a Manager and instantiates platform adapters.
func NewManager(cfg Config) *Manager {
	m := &Manager{gateways: make(map[string]Gateway)}
	for _, entry := range cfg.Accounts {
		ac := AccountConfig{
			Broker:   entry.Broker,
			Platform: entry.Platform,
			Login:    entry.Login,
			Password: entry.Password,
			Server:   entry.Server,
			Host:     entry.Host,
			Port:     entry.Port,
		}
		key := entry.Broker + "-" + entry.Login
		m.gateways[key] = newGateway(ac, m.normalizer)
	}
	return m
}

// AddGateway adds or replaces a gateway connection.
func (m *Manager) AddGateway(cfg AccountConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := cfg.Broker + "-" + cfg.Login
	m.gateways[key] = newGateway(cfg, m.normalizer)
}

// RemoveGateway removes a gateway connection.
func (m *Manager) RemoveGateway(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.gateways, key)
}

func newGateway(cfg AccountConfig, normalizer *Normalizer) Gateway {
	switch cfg.Platform {
	case "mt4":
		return newMT4Gateway(cfg, normalizer)
	case "mt5":
		return newMT5Gateway(cfg, normalizer)
	default:
		panic(fmt.Sprintf("mdgateway: unknown platform %q", cfg.Platform))
	}
}

// Connections returns all managed gateways.
func (m *Manager) Connections() map[string]Gateway {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]Gateway, len(m.gateways))
	for k, v := range m.gateways {
		out[k] = v
	}
	return out
}
