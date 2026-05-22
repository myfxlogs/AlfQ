package factorsvc

import (
	"context"
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"go.uber.org/zap"
)

func TestWindowBufferPush(t *testing.T) {
	wb := NewWindowBuffer(5, zap.NewNop())

	// Push 3 bars
	for i := 0; i < 3; i++ {
		bar := &pb.Bar{
			TenantId:      "t1",
			Symbol:        "EURUSD",
			Period:        "M1",
			CloseTsUnixMs: int64(i * 60000),
		}
		bar.Open = &pb.Money{Value: "1.0"}
		bar.High = &pb.Money{Value: "1.0"}
		bar.Low = &pb.Money{Value: "1.0"}
		bar.Close = &pb.Money{Value: "1.0"}
		wb.Push("t1", "EURUSD", "M1", bar)
	}

	bars := wb.Snapshot("t1", "EURUSD", "M1", 10)
	if len(bars) != 3 {
		t.Fatalf("expected 3 bars, got %d", len(bars))
	}
}

func TestWindowBufferEviction(t *testing.T) {
	wb := NewWindowBuffer(3, zap.NewNop())

	for i := 0; i < 5; i++ {
		bar := &pb.Bar{
			TenantId:      "t1",
			Symbol:        "EURUSD",
			Period:        "M1",
			CloseTsUnixMs: int64(i * 60000),
		}
		bar.Open = &pb.Money{Value: "1.0"}
		bar.High = &pb.Money{Value: "1.0"}
		bar.Low = &pb.Money{Value: "1.0"}
		bar.Close = &pb.Money{Value: "1.0"}
		wb.Push("t1", "EURUSD", "M1", bar)
	}

	bars := wb.Snapshot("t1", "EURUSD", "M1", 10)
	if len(bars) != 3 {
		t.Fatalf("expected 3 bars after eviction, got %d", len(bars))
	}
}

func TestWindowBufferBootstrap(t *testing.T) {
	wb := NewWindowBuffer(10, zap.NewNop())
	specs := []BootstrapSpec{
		{TenantID: "t1", Symbol: "EURUSD", Period: "D1", Limit: 20},
	}
	wb.Bootstrap(context.Background(), specs)

	bars := wb.Snapshot("t1", "EURUSD", "D1", 10)
	// Bootstrap only pre-allocates; no live data yet
	if len(bars) != 0 {
		t.Fatalf("expected 0 bars after bootstrap, got %d", len(bars))
	}
}

func TestWindowBufferIsolation(t *testing.T) {
	wb := NewWindowBuffer(10, zap.NewNop())

	bar1 := &pb.Bar{TenantId: "t1", Symbol: "EURUSD", Period: "M1", CloseTsUnixMs: 1}
	bar1.Open = &pb.Money{Value: "1.0"}
	bar1.Close = &pb.Money{Value: "1.0"}
	bar1.High = &pb.Money{Value: "1.0"}
	bar1.Low = &pb.Money{Value: "1.0"}

	bar2 := &pb.Bar{TenantId: "t2", Symbol: "EURUSD", Period: "M1", CloseTsUnixMs: 1}
	bar2.Open = &pb.Money{Value: "2.0"}
	bar2.Close = &pb.Money{Value: "2.0"}
	bar2.High = &pb.Money{Value: "2.0"}
	bar2.Low = &pb.Money{Value: "2.0"}

	wb.Push("t1", "EURUSD", "M1", bar1)
	wb.Push("t2", "EURUSD", "M1", bar2)

	bars1 := wb.Snapshot("t1", "EURUSD", "M1", 10)
	bars2 := wb.Snapshot("t2", "EURUSD", "M1", 10)

	if len(bars1) != 1 || len(bars2) != 1 {
		t.Fatal("tenant isolation broken")
	}
}
