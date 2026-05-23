package risksvc

import (
	"context"
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func TestKillSwitchDefaultInactive(t *testing.T) {
	k := &KillSwitch{}
	if k.IsActive() {
		t.Fatal("expected inactive")
	}
}

func TestKillSwitchActivateDeactivate(t *testing.T) {
	k := &KillSwitch{}
	k.Activate("global", "admin", "test")
	if !k.IsActive() {
		t.Fatal("expected active")
	}
	active, scope, by, reason := k.Status()
	if !active || scope != "global" || by != "admin" || reason != "test" {
		t.Fatalf("Status mismatch")
	}
	k.Deactivate()
	if k.IsActive() {
		t.Fatal("expected inactive")
	}
}

func TestBreakerAllowAndOpen(t *testing.T) {
	b := NewBreaker(3)
	for i := 0; i < 3; i++ {
		b.RecordFailure()
	}
	// Allow should still return true until Allow() is called after failures >= maxFailures
	if b.Allow() { // open was set by 3rd RecordFailure → failures=3 >= max=3 → open=true
		t.Fatal("should be open")
	}
}

func TestBreakerRecordFailure(t *testing.T) {
	b := NewBreaker(2)
	b.RecordFailure()
	if !b.Allow() {
		t.Fatal("still open after 1")
	}
	b.RecordFailure()
	if b.Allow() {
		t.Fatal("should be open")
	}
}

func TestBreakerReset(t *testing.T) {
	b := NewBreaker(2)
	b.RecordFailure()
	b.RecordFailure()
	b.Reset()
	if !b.Allow() {
		t.Fatal("should be closed after reset")
	}
}

func TestEngineDefaultRules(t *testing.T) {
	e := NewEngine()
	// NewEngine registers 10 default rules
	n := len(e.rules)
	if n < 5 {
		t.Fatal("rule count mismatch")
	}
}

func TestEngineCheckNoRules(t *testing.T) {
	e := NewEngine()
	if e.Check(context.Background(), &pb.OrderRequest{}) == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestMaxLotCheck(t *testing.T) {
	r := &MaxLot{maxLot: 1.0}
	res := r.Check(context.Background(), &pb.OrderRequest{Qty: 2.0}, &AccountState{})
	if res.Approved {
		t.Fatal("expected rejected")
	}
	res = r.Check(context.Background(), &pb.OrderRequest{Qty: 0.5}, &AccountState{})
	if !res.Approved {
		t.Fatal("expected approved")
	}
}

func TestMaxPositionCheck(t *testing.T) {
	r := &MaxPosition{maxPerSymbol: 1.0}
	state := &AccountState{Positions: map[string]*pb.Position{
		"EURUSD": {Qty: 0.5},
	}}
	res := r.Check(context.Background(), &pb.OrderRequest{Symbol: "EURUSD", Qty: 0.3, Side: pb.OrderSide_ORDER_SIDE_BUY}, state)
	if !res.Approved {
		t.Fatalf("expected approved, got: %s", res.Reason)
	}
	res = r.Check(context.Background(), &pb.OrderRequest{Symbol: "EURUSD", Qty: 1.0, Side: pb.OrderSide_ORDER_SIDE_BUY}, state)
	if res.Approved {
		t.Fatal("expected rejected: exceeds limit")
	}
}

func TestCanonicalAuthNoPG(t *testing.T) {
	// Without PG pool, CanonicalAuth allows all (development mode)
	r := NewCanonicalAuth(nil)
	res := r.Check(context.Background(), &pb.OrderRequest{Symbol: "EURUSD", StrategyId: "s1", TenantId: "t1"}, &AccountState{})
	if !res.Approved {
		t.Fatal("EURUSD should be allowed in dev mode (no PG)")
	}
	res = r.Check(context.Background(), &pb.OrderRequest{Symbol: "BTCUSD", StrategyId: "s2", TenantId: "t2"}, &AccountState{})
	if !res.Approved {
		t.Fatal("BTCUSD should be allowed in dev mode (no PG)")
	}
}

func TestDailyLossCheck(t *testing.T) {
	r := &DailyLoss{maxDailyLoss: 1000}
	state := &AccountState{DailyPnL: -500}
	res := r.Check(context.Background(), &pb.OrderRequest{}, state)
	if !res.Approved {
		t.Fatalf("expected approved, reason=%s", res.Reason)
	}
	state.DailyPnL = -1200
	res = r.Check(context.Background(), &pb.OrderRequest{}, state)
	if res.Approved {
		t.Fatal("expected rejected")
	}
}

func TestDrawdownCheck(t *testing.T) {
	r := &Drawdown{maxDrawdown: 0.2}
	state := &AccountState{MaxDrawdown: 0.1}
	res := r.Check(context.Background(), &pb.OrderRequest{}, state)
	if !res.Approved {
		t.Fatal("expected approved")
	}
	state.MaxDrawdown = 0.3
	res = r.Check(context.Background(), &pb.OrderRequest{}, state)
	if res.Approved {
		t.Fatal("expected rejected")
	}
}
