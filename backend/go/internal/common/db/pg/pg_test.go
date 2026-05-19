package pg_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alfq/backend/go/internal/common/db/pg"
)

func TestConnectInvalidDSN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := pg.Connect(ctx, "invalid-dsn-format")
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
	if !strings.Contains(err.Error(), "pg:") {
		t.Fatalf("expected pg: prefix in error, got: %v", err)
	}
}

func TestConnectUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := pg.Connect(ctx, "postgres://no:body@localhost:19999/nodb?connect_timeout=2")
	if err == nil {
		t.Skip("unexpectedly connected")
	}
	if !strings.Contains(err.Error(), "pg:") {
		t.Fatalf("expected pg: prefix in error, got: %v", err)
	}
}

func TestPoolCloseNilSafe(t *testing.T) {
	// Close on nil pointer should not panic (safety check)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Close panicked: %v", r)
		}
	}()
	var p *pg.Pool
	p.Close() // p is nil, p.Close() should not dereference
}
