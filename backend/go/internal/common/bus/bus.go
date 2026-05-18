// Package bus provides NATS JetStream publish/subscribe utilities.
package bus

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
)

// Client wraps NATS connection with JetStream helpers.
type Client struct {
	nc *nats.Conn
	js nats.JetStreamContext
}

// Connect creates a NATS client with JetStream.
func Connect(ctx context.Context, url string) (*Client, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("bus: connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("bus: jetstream: %w", err)
	}
	return &Client{nc: nc, js: js}, nil
}

// Publish sends a message to a JetStream subject.
func (c *Client) Publish(ctx context.Context, subject string, data []byte) error {
	_, err := c.js.Publish(subject, data)
	return err
}

// Subscribe creates a subscription with JetStream.
func (c *Client) Subscribe(ctx context.Context, subject string, handler func(data []byte)) (*nats.Subscription, error) {
	return c.js.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Data)
	})
}

// Close releases the connection.
func (c *Client) Close() { c.nc.Close() }
