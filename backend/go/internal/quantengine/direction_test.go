package quantengine

import (
	"testing"
)

func TestDirection(t *testing.T) {
	// Test Direction function with various signal values
	direction := Direction(1.0)
	if direction != "long" {
		t.Fatalf("expected long, got %s", direction)
	}

	direction = Direction(-1.0)
	if direction != "short" {
		t.Fatalf("expected short, got %s", direction)
	}

	direction = Direction(0.0)
	if direction != "flat" {
		t.Fatalf("expected flat, got %s", direction)
	}
}
