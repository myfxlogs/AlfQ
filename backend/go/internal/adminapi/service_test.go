package adminapi

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/auth"
)

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestNewAdapter(t *testing.T) {
	svc := NewService(nil)
	adp := NewAdapter(svc)
	if adp == nil {
		t.Fatal("NewAdapter returned nil")
	}
	if adp.svc != svc {
		t.Fatal("adapter svc mismatch")
	}
}

func TestEffectiveTenantIDFromRequest(t *testing.T) {
	ctx := context.Background()
	tid := effectiveTenantID(ctx, "aaaa-bbbb-cccc")
	if tid != "aaaa-bbbb-cccc" {
		t.Fatalf("effectiveTenantID req: got %q", tid)
	}
}

func TestEffectiveTenantIDFromContext(t *testing.T) {
	ctx := auth.WithTenant(context.Background(), "ctx-tenant-999")
	tid := effectiveTenantID(ctx, "")
	if tid != "ctx-tenant-999" {
		t.Fatalf("effectiveTenantID ctx: got %q", tid)
	}
}

func TestEffectiveTenantIDEmpty(t *testing.T) {
	ctx := context.Background()
	tid := effectiveTenantID(ctx, "")
	if tid != "" {
		t.Fatalf("effectiveTenantID empty: got %q, want empty", tid)
	}
}

func TestSHA256Hex(t *testing.T) {
	result := sha256Hex("hello")
	if len(result) != 64 {
		t.Fatalf("sha256Hex length: got %d, want 64", len(result))
	}
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if result != expected {
		t.Fatalf("sha256Hex mismatch: got %q, want %q", result, expected)
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	tok, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("generateRefreshToken: %v", err)
	}
	if len(tok) != 64 {
		t.Fatalf("refresh token length: got %d, want 64", len(tok))
	}
	tok2, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("generateRefreshToken 2: %v", err)
	}
	if tok == tok2 {
		t.Fatal("expected different refresh tokens")
	}
}

// -- Service stub method tests (no DB needed) --

func TestServiceListBacktests(t *testing.T) {
	svc := NewService(nil)
	resp, err := svc.ListBacktests(context.Background(), &pb.ListBacktestsRequest{})
	if err != nil {
		t.Fatalf("ListBacktests: %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
}

func TestServiceListAuditLogs(t *testing.T) {
	svc := NewService(nil)
	resp, err := svc.ListAuditLogs(context.Background(), &pb.ListAuditLogsRequest{})
	if err != nil {
		t.Fatalf("ListAuditLogs: %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
}

// -- Adapter stub method tests --

func TestAdapterStreamSignals(t *testing.T) {
	adp := NewAdapter(NewService(nil))
	req := connect.NewRequest(&pb.StreamSignalsRequest{})
	stream := &connect.ServerStream[pb.Signal]{}
	err := adp.StreamSignals(context.Background(), req, stream)
	if err != nil {
		t.Fatalf("StreamSignals: %v", err)
	}
}

func TestAdapterRunBacktest(t *testing.T) {
	adp := NewAdapter(NewService(nil))
	req := connect.NewRequest(&pb.RunBacktestRequest{})
	stream := &connect.ServerStream[pb.BacktestProgress]{}
	err := adp.RunBacktest(context.Background(), req, stream)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
}

func TestAdapterStreamAuditLogs(t *testing.T) {
	adp := NewAdapter(NewService(nil))
	req := connect.NewRequest(&pb.StreamAuditLogsRequest{})
	stream := &connect.ServerStream[pb.AuditLog]{}
	err := adp.StreamAuditLogs(context.Background(), req, stream)
	if err != nil {
		t.Fatalf("StreamAuditLogs: %v", err)
	}
}

func TestAdapterListBacktests(t *testing.T) {
	adp := NewAdapter(NewService(nil))
	req := connect.NewRequest(&pb.ListBacktestsRequest{})
	resp, err := adp.ListBacktests(context.Background(), req)
	if err != nil {
		t.Fatalf("ListBacktests: %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
}

func TestAdapterListAuditLogs(t *testing.T) {
	adp := NewAdapter(NewService(nil))
	req := connect.NewRequest(&pb.ListAuditLogsRequest{})
	resp, err := adp.ListAuditLogs(context.Background(), req)
	if err != nil {
		t.Fatalf("ListAuditLogs: %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
}