// Package oms implements the Order Management System.
//
// Order state machine per docs/14-领域模型与交易规则.md §3.1.
// Terminal states: CANCELLED, REJECTED, FAILED, FILLED, EXPIRED.

package oms

import (
	"fmt"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Transition validates and executes an order state transition.
// Returns an error if the transition is invalid.
func Transition(current, next pb.OrderState) error {
	if !isValid(current, next) {
		return fmt.Errorf("oms: invalid transition %s → %s", current, next)
	}
	return nil
}

// IsTerminal returns true if the state is a final state.
func IsTerminal(s pb.OrderState) bool {
	switch s {
	case pb.OrderState_ORDER_STATE_CANCELLED,
		pb.OrderState_ORDER_STATE_REJECTED,
		pb.OrderState_ORDER_STATE_FAILED,
		pb.OrderState_ORDER_STATE_FILLED,
		pb.OrderState_ORDER_STATE_EXPIRED:
		return true
	}
	return false
}

// isValid defines the allowed state transitions.
//
//	NEW → VALIDATED → RISK_APPROVED → SUBMITTED
//	                                    ├── WORKING
//	                                    ├── PARTIALLY_FILLED → FILLED
//	                                    ├── FILLED
//	                                    ├── CANCELLED
//	                                    ├── EXPIRED
//	                                    └── FAILED
//	VALIDATED → REJECTED
//	RISK_APPROVED → REJECTED
func isValid(current, next pb.OrderState) bool {
	transitions := map[pb.OrderState][]pb.OrderState{
		pb.OrderState_ORDER_STATE_NEW: {
			pb.OrderState_ORDER_STATE_VALIDATED,
		},
		pb.OrderState_ORDER_STATE_VALIDATED: {
			pb.OrderState_ORDER_STATE_RISK_APPROVED,
			pb.OrderState_ORDER_STATE_REJECTED,
		},
		pb.OrderState_ORDER_STATE_RISK_APPROVED: {
			pb.OrderState_ORDER_STATE_SUBMITTED,
			pb.OrderState_ORDER_STATE_REJECTED,
		},
		pb.OrderState_ORDER_STATE_SUBMITTED: {
			pb.OrderState_ORDER_STATE_WORKING,
			pb.OrderState_ORDER_STATE_PARTIALLY_FILLED,
			pb.OrderState_ORDER_STATE_FILLED,
			pb.OrderState_ORDER_STATE_CANCELLED,
			pb.OrderState_ORDER_STATE_EXPIRED,
			pb.OrderState_ORDER_STATE_FAILED,
		},
		pb.OrderState_ORDER_STATE_WORKING: {
			pb.OrderState_ORDER_STATE_PARTIALLY_FILLED,
			pb.OrderState_ORDER_STATE_FILLED,
			pb.OrderState_ORDER_STATE_CANCELLED,
			pb.OrderState_ORDER_STATE_EXPIRED,
			pb.OrderState_ORDER_STATE_FAILED,
		},
		pb.OrderState_ORDER_STATE_PARTIALLY_FILLED: {
			pb.OrderState_ORDER_STATE_PARTIALLY_FILLED,
			pb.OrderState_ORDER_STATE_FILLED,
			pb.OrderState_ORDER_STATE_CANCELLED,
			pb.OrderState_ORDER_STATE_EXPIRED,
			pb.OrderState_ORDER_STATE_FAILED,
		},
	}

	allowed, ok := transitions[current]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}
