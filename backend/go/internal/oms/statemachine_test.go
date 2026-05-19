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
		{pb.OrderState_ORDER_STATE_NEW, pb.OrderState_ORDER_STATE_SUBMITTED},      // skip validated
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
