package symbolsync

import (
	"testing"
)

func TestNewRepo(t *testing.T) {
	r := NewRepo(nil)
	if r == nil {
		t.Fatal("NewRepo returned nil")
	}
}
