package mthub

import (
	"sync"

	mthubv1 "github.com/alfq/backend/go/gen/alfq/mthub/v1"
)

// OrderEventBroker multiplexes order events from a single MT session
// to multiple gRPC streaming subscribers.
type OrderEventBroker struct {
	mu          sync.RWMutex
	subscribers map[string]chan *mthubv1.OrderEvent // accountID → event chan
}

// NewOrderEventBroker creates a new event broker.
func NewOrderEventBroker() *OrderEventBroker {
	return &OrderEventBroker{
		subscribers: make(map[string]chan *mthubv1.OrderEvent),
	}
}

// Subscribe registers a subscriber for the given account and returns
// a channel that receives OrderEvents. Call Unsubscribe when done.
func (b *OrderEventBroker) Subscribe(accountID string) chan *mthubv1.OrderEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan *mthubv1.OrderEvent, 64)
	b.subscribers[accountID] = ch
	return ch
}

// Unsubscribe removes a subscriber. The caller should close the channel.
func (b *OrderEventBroker) Unsubscribe(accountID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subscribers[accountID]; ok {
		delete(b.subscribers, accountID)
		close(ch)
	}
}

// Publish sends an event to the subscriber for the given account.
// Non-blocking — drops if the subscriber's buffer is full.
func (b *OrderEventBroker) Publish(ev *mthubv1.OrderEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	ch, ok := b.subscribers[ev.AccountId]
	if !ok {
		return
	}
	select {
	case ch <- ev:
	default:
		// drop if channel is full — subscriber is too slow
	}
}

// SubscriberCount returns the number of active subscribers.
func (b *OrderEventBroker) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
