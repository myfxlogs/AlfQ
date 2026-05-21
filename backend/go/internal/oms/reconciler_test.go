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
