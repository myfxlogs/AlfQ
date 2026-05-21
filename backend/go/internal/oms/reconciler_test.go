package oms

import (
	"testing"
)

func TestNewReconciler(t *testing.T) {
	r := NewReconciler(nil, nil)
	if r == nil {
		t.Fatal("NewReconciler returned nil")
	}
}

func TestReconciler_Fields(t *testing.T) {
	r := NewReconciler(nil, nil)
	if r.orders != nil {
		t.Fatal("expected orders to be nil")
	}
	if r.adapter != nil {
		t.Fatal("expected adapter to be nil")
	}
}
