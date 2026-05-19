package risksvc

import (
	"testing"
)

func TestKillSwitchInit(t *testing.T) {
	ks := &KillSwitch{}
	if ks.IsActive() {
		t.Fatal("kill switch should not be active initially")
	}
}

func TestBreakerExhaust(t *testing.T) {
	b := NewBreaker(2)
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()
	if b.Allow() {
		t.Fatal("breaker should deny after failures exceed max")
	}
}

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("nil engine")
	}
}

func TestEventRecorder(t *testing.T) {
	r := NewEventRecorder()
	if r == nil {
		t.Fatal("nil event recorder")
	}
}

func TestKillExecutor(t *testing.T) {
	e := NewKillExecutor()
	if e == nil {
		t.Fatal("nil kill executor")
	}
}
