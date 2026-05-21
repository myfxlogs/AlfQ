package oms_test

import (
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/oms"
)

func TestTerminalStates(t *testing.T) {
	terminals := []pb.OrderState{
		pb.OrderState_ORDER_STATE_CANCELLED,
		pb.OrderState_ORDER_STATE_REJECTED,
		pb.OrderState_ORDER_STATE_FAILED,
		pb.OrderState_ORDER_STATE_FILLED,
		pb.OrderState_ORDER_STATE_EXPIRED,
	}
	for _, s := range terminals {
		if !oms.IsTerminal(s) {
			t.Errorf("expected %s to be terminal", s)
		}
	}
}

func TestNonTerminalStates(t *testing.T) {
	nonTerminals := []pb.OrderState{
		pb.OrderState_ORDER_STATE_NEW,
		pb.OrderState_ORDER_STATE_VALIDATED,
		pb.OrderState_ORDER_STATE_RISK_APPROVED,
		pb.OrderState_ORDER_STATE_SUBMITTED,
		pb.OrderState_ORDER_STATE_WORKING,
		pb.OrderState_ORDER_STATE_PARTIALLY_FILLED,
	}
	for _, s := range nonTerminals {
		if oms.IsTerminal(s) {
			t.Errorf("expected %s to be non-terminal", s)
		}
	}
}

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		from, to pb.OrderState
	}{
		{pb.OrderState_ORDER_STATE_NEW, pb.OrderState_ORDER_STATE_VALIDATED},
		{pb.OrderState_ORDER_STATE_VALIDATED, pb.OrderState_ORDER_STATE_RISK_APPROVED},
		{pb.OrderState_ORDER_STATE_RISK_APPROVED, pb.OrderState_ORDER_STATE_SUBMITTED},
		{pb.OrderState_ORDER_STATE_SUBMITTED, pb.OrderState_ORDER_STATE_WORKING},
		{pb.OrderState_ORDER_STATE_WORKING, pb.OrderState_ORDER_STATE_PARTIALLY_FILLED},
		{pb.OrderState_ORDER_STATE_PARTIALLY_FILLED, pb.OrderState_ORDER_STATE_FILLED},
		{pb.OrderState_ORDER_STATE_SUBMITTED, pb.OrderState_ORDER_STATE_CANCELLED},
		{pb.OrderState_ORDER_STATE_WORKING, pb.OrderState_ORDER_STATE_CANCELLED},
		{pb.OrderState_ORDER_STATE_VALIDATED, pb.OrderState_ORDER_STATE_REJECTED},
	}
	for _, tt := range tests {
		if err := oms.Transition(tt.from, tt.to); err != nil {
			t.Errorf("transition %s → %s should be valid: %v", tt.from, tt.to, err)
		}
	}
}

func TestInvalidTransitions(t *testing.T) {
	tests := []struct {
		from, to pb.OrderState
	}{
		{pb.OrderState_ORDER_STATE_NEW, pb.OrderState_ORDER_STATE_SUBMITTED},       // skip validated
		{pb.OrderState_ORDER_STATE_FILLED, pb.OrderState_ORDER_STATE_WORKING},      // terminal → non-terminal
		{pb.OrderState_ORDER_STATE_CANCELLED, pb.OrderState_ORDER_STATE_SUBMITTED}, // terminal → non-terminal
		{pb.OrderState_ORDER_STATE_NEW, pb.OrderState_ORDER_STATE_FILLED},          // skip all
	}
	for _, tt := range tests {
		if err := oms.Transition(tt.from, tt.to); err == nil {
			t.Errorf("transition %s → %s should be invalid", tt.from, tt.to)
		}
	}
}

func TestTransition_Valid(t *testing.T) {
	tests := []struct {
		current pb.OrderState
		next    pb.OrderState
		wantErr bool
	}{
		{pb.OrderState_ORDER_STATE_NEW, pb.OrderState_ORDER_STATE_VALIDATED, false},
		{pb.OrderState_ORDER_STATE_VALIDATED, pb.OrderState_ORDER_STATE_RISK_APPROVED, false},
		{pb.OrderState_ORDER_STATE_RISK_APPROVED, pb.OrderState_ORDER_STATE_SUBMITTED, false},
		{pb.OrderState_ORDER_STATE_SUBMITTED, pb.OrderState_ORDER_STATE_WORKING, false},
		{pb.OrderState_ORDER_STATE_SUBMITTED, pb.OrderState_ORDER_STATE_FILLED, false},
		{pb.OrderState_ORDER_STATE_WORKING, pb.OrderState_ORDER_STATE_FILLED, false},
		{pb.OrderState_ORDER_STATE_PARTIALLY_FILLED, pb.OrderState_ORDER_STATE_FILLED, false},
		{pb.OrderState_ORDER_STATE_SUBMITTED, pb.OrderState_ORDER_STATE_CANCELLED, false},
		{pb.OrderState_ORDER_STATE_VALIDATED, pb.OrderState_ORDER_STATE_REJECTED, false},
		{pb.OrderState_ORDER_STATE_RISK_APPROVED, pb.OrderState_ORDER_STATE_REJECTED, false},
		{pb.OrderState_ORDER_STATE_NEW, pb.OrderState_ORDER_STATE_FILLED, true}, // invalid
		{pb.OrderState_ORDER_STATE_FILLED, pb.OrderState_ORDER_STATE_WORKING, true}, // invalid
	}

	for _, tt := range tests {
		t.Run(tt.current.String()+"→"+tt.next.String(), func(t *testing.T) {
			err := oms.Transition(tt.current, tt.next)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		state pb.OrderState
		want  bool
	}{
		{pb.OrderState_ORDER_STATE_CANCELLED, true},
		{pb.OrderState_ORDER_STATE_REJECTED, true},
		{pb.OrderState_ORDER_STATE_FAILED, true},
		{pb.OrderState_ORDER_STATE_FILLED, true},
		{pb.OrderState_ORDER_STATE_EXPIRED, true},
		{pb.OrderState_ORDER_STATE_NEW, false},
		{pb.OrderState_ORDER_STATE_VALIDATED, false},
		{pb.OrderState_ORDER_STATE_WORKING, false},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := oms.IsTerminal(tt.state); got != tt.want {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTransition_MoreCases(t *testing.T) {
	// Test additional valid transitions
	err := oms.Transition(pb.OrderState_ORDER_STATE_SUBMITTED, pb.OrderState_ORDER_STATE_PARTIALLY_FILLED)
	if err != nil {
		t.Errorf("SUBMITTED → PARTIALLY_FILLED should be valid, got error: %v", err)
	}

	err = oms.Transition(pb.OrderState_ORDER_STATE_PARTIALLY_FILLED, pb.OrderState_ORDER_STATE_PARTIALLY_FILLED)
	if err != nil {
		t.Errorf("PARTIALLY_FILLED → PARTIALLY_FILLED should be valid, got error: %v", err)
	}

	err = oms.Transition(pb.OrderState_ORDER_STATE_WORKING, pb.OrderState_ORDER_STATE_CANCELLED)
	if err != nil {
		t.Errorf("WORKING → CANCELLED should be valid, got error: %v", err)
	}

	err = oms.Transition(pb.OrderState_ORDER_STATE_SUBMITTED, pb.OrderState_ORDER_STATE_EXPIRED)
	if err != nil {
		t.Errorf("SUBMITTED → EXPIRED should be valid, got error: %v", err)
	}

	// Test invalid transitions
	err = oms.Transition(pb.OrderState_ORDER_STATE_CANCELLED, pb.OrderState_ORDER_STATE_WORKING)
	if err == nil {
		t.Error("CANCELLED → WORKING should be invalid")
	}

	err = oms.Transition(pb.OrderState_ORDER_STATE_REJECTED, pb.OrderState_ORDER_STATE_SUBMITTED)
	if err == nil {
		t.Error("REJECTED → SUBMITTED should be invalid")
	}
}
