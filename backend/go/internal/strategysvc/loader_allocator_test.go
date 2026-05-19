package strategysvc

import (
	"context"
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

type stubStrategy struct{}

func (s *stubStrategy) ID() string { return "stub" }
func (s *stubStrategy) OnFactor(ctx context.Context, f string, v float64) (*Signal, error) {
	return nil, nil
}
func (s *stubStrategy) OnBar(ctx context.Context, bar *pb.Bar) (*Signal, error) {
	return nil, nil
}

func TestLoaderDeployAndGet(t *testing.T) {
	l := NewLoader()
	l.Deploy("d1", &stubStrategy{})
	r := l.Get("d1")
	if r == nil {
		t.Fatal("Get returned nil after Deploy")
	}
}

func TestLoaderUndeploy(t *testing.T) {
	l := NewLoader()
	l.Deploy("d1", &stubStrategy{})
	l.Undeploy("d1")
	if l.Get("d1") != nil {
		t.Fatal("Get should return nil after Undeploy")
	}
}

func TestLoaderListAndCount(t *testing.T) {
	l := NewLoader()
	l.Deploy("a", &stubStrategy{})
	l.Deploy("b", &stubStrategy{})
	if l.Count() != 2 {
		t.Fatalf("Count: got %d", l.Count())
	}
	l.Undeploy("a")
	if l.Count() != 1 {
		t.Fatalf("Count after undeploy: got %d", l.Count())
	}
}

func TestLoaderGetStatus(t *testing.T) {
	l := NewLoader()
	l.Deploy("d1", &stubStrategy{})
	st := l.GetStatus("d1")
	if !st.Active {
		t.Fatal("expected active")
	}
	st = l.GetStatus("missing")
	if st.Active {
		t.Fatal("expected inactive for missing")
	}
}

func TestLoaderHotReload(t *testing.T) {
	l := NewLoader()
	l.Deploy("d1", &stubStrategy{})
	l.HotReload(context.Background(), "d1", &stubStrategy{})
}

func TestAllocatorSetAccount(t *testing.T) {
	a := NewAllocator()
	a.SetAccount("acc1", 10000)
	s := a.Summary("acc1")
	if s == nil || s.TotalEquity != 10000 {
		t.Fatal("SetAccount failed")
	}
}

func TestAllocatorAddRemoveStrategy(t *testing.T) {
	a := NewAllocator()
	a.AddStrategy("acc1", "strat1", 0.3, 5.0, 0.1)
	if v := a.MaxOrderSize("acc1", "strat1"); v != 5.0 {
		t.Fatalf("MaxOrderSize: got %f", v)
	}
	a.RemoveStrategy("acc1", "strat1")
	if a.MaxOrderSize("acc1", "strat1") != 0 {
		t.Fatal("MaxOrderSize should be 0 after remove")
	}
}

func TestAllocatorMaxOrderSizeUnknown(t *testing.T) {
	a := NewAllocator()
	if a.MaxOrderSize("x", "y") != 0 {
		t.Fatal("expected 0")
	}
}

func TestAllocatorSummaryNil(t *testing.T) {
	a := NewAllocator()
	if a.Summary("nonexistent") != nil {
		t.Fatal("expected nil")
	}
}