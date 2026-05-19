// Package ch provides ClickHouse client utilities.
package ch

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Client wraps a ClickHouse connection handle.
type Client struct {
	conn driver.Conn
}

// Connect creates a new ClickHouse client and pings the server.
func Connect(ctx context.Context, addr, database string) (*Client, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: database,
		},
		DialTimeout:      10 * time.Second,
		MaxOpenConns:     10,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Hour,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("ch: open: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ch: ping: %w", err)
	}
	return &Client{conn: conn}, nil
}

// Insert writes a batch of rows using the Native protocol.
// rows is a slice of maps, each keyed by column name.
// All rows must share the same set of keys; column order is determined
// by the first row's sorted key set.
func (c *Client) Insert(ctx context.Context, table string, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}

	// Derive stable column order from first row
	cols := sortedKeys(rows[0])
	batch, err := c.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s (%s)", table, joinCols(cols)))
	if err != nil {
		return fmt.Errorf("ch: prepare batch: %w", err)
	}
	for _, row := range rows {
		vals := make([]any, len(cols))
		for i, col := range cols {
			vals[i] = row[col]
		}
		if err := batch.Append(vals...); err != nil {
			return fmt.Errorf("ch: append row: %w", err)
		}
	}
	return batch.Send()
}

// Ping checks connectivity to the ClickHouse server.
func (c *Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

// Close releases the client and underlying connection pool.
func (c *Client) Close() error {
	return c.conn.Close()
}

// sortedKeys returns the sorted key set of a map.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// joinCols returns a comma-separated, double-quoted column list.
func joinCols(cols []string) string {
	s := ""
	for i, c := range cols {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf(`"%s"`, c)
	}
	return s
}
