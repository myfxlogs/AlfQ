// Package mdgateway — ClickHouse connection management.
package mdgateway

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.uber.org/zap"
)

// CHConnConfig holds ClickHouse connection settings.
type CHConnConfig struct {
	Addr     string // host:port (default "localhost:9000")
	Database string // database name (default "alfq")
	User     string // (default "alfq")
	Password string // (default empty — dev)
}

// DefaultCHConnConfig returns sensible dev defaults.
func DefaultCHConnConfig() CHConnConfig {
	return CHConnConfig{
		Addr:     "localhost:9000",
		Database: "alfq",
		User:     "alfq",
		Password: "",
	}
}

// CHConn wraps a native ClickHouse connection with auto-reconnect.
type CHConn struct {
	cfg CHConnConfig
	log *zap.Logger
	conn clickhouse.Conn
}

// NewCHConn creates a ClickHouse connection manager.
func NewCHConn(cfg CHConnConfig, log *zap.Logger) (*CHConn, error) {
	c := &CHConn{cfg: cfg, log: log}
	if err := c.connect(context.Background()); err != nil {
		return nil, fmt.Errorf("chconn: initial connect: %w", err)
	}
	return c, nil
}

// Conn returns the underlying ClickHouse connection, reconnecting if needed.
func (c *CHConn) Conn(ctx context.Context) (clickhouse.Conn, error) {
	if c.conn != nil {
		if err := c.conn.Ping(ctx); err == nil {
			return c.conn, nil
		}
		c.log.Warn("chconn: ping failed, reconnecting")
	}
	if err := c.connect(ctx); err != nil {
		return nil, err
	}
	return c.conn, nil
}

func (c *CHConn) connect(ctx context.Context) error {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{c.cfg.Addr},
		Auth: clickhouse.Auth{
			Database: c.cfg.Database,
			Username: c.cfg.User,
			Password: c.cfg.Password,
		},
		DialTimeout:     5 * time.Second,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		return fmt.Errorf("chconn: open: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("chconn: ping: %w", err)
	}
	c.conn = conn
	c.log.Info("chconn: connected", zap.String("addr", c.cfg.Addr))
	return nil
}

// Close releases the connection.
func (c *CHConn) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
