package oms

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewReconciler(t *testing.T) {
	r := NewReconciler(nil, nil, nil, zap.NewNop())
	if r == nil {
		t.Fatal("NewReconciler returned nil")
	}
}

func TestReconciler_Fields(t *testing.T) {
	r := NewReconciler(nil, nil, nil, zap.NewNop())
	if r.orders != nil {
		t.Fatal("expected orders to be nil")
	}
	if r.pool != nil {
		t.Fatal("expected pool to be nil")
	}
}
