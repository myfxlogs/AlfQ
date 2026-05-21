package bus

import (
	"context"
	"testing"
)

func TestConnect_InvalidURL(t *testing.T) {
	ctx := context.Background()
	_, err := Connect(ctx, "invalid://url")
	if err == nil {
		t.Fatal("Connect with invalid URL should fail")
	}
}

func TestClient_Close(t *testing.T) {
	// Close should not panic on nil client
	c := &Client{}
	c.Close()
}
