// Package risksvc — Kill Switch implementation.
package risksvc

import (
	"sync"
)

// KillSwitch provides emergency stop functionality.
type KillSwitch struct {
	mu     sync.RWMutex
	active bool
	scope  string
	byUser string
	reason string
}

// Activate triggers the kill switch. All new orders will be rejected.
func (k *KillSwitch) Activate(scope, byUser, reason string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.active = true
	k.scope = scope
	k.byUser = byUser
	k.reason = reason
}

// Deactivate releases the kill switch.
func (k *KillSwitch) Deactivate() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.active = false
}

// IsActive returns true if the kill switch is engaged.
func (k *KillSwitch) IsActive() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.active
}

// Status returns the current kill switch status.
func (k *KillSwitch) Status() (active bool, scope, byUser, reason string) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.active, k.scope, k.byUser, k.reason
}

// Breaker implements a circuit breaker for order rate limiting.
type Breaker struct {
	mu              sync.Mutex
	failures        int
	maxFailures     int
	open            bool
	rejectThreshold int
}

// NewBreaker creates a circuit breaker.
func NewBreaker(maxFailures int) *Breaker {
	return &Breaker{maxFailures: maxFailures, rejectThreshold: maxFailures}
}

// Allow returns true if the request should be allowed through.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.open {
		return false
	}
	if b.failures >= b.maxFailures {
		b.open = true
		return false
	}
	return true
}

// RecordFailure records a failure and potentially opens the breaker.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	b.failures++
	if b.failures >= b.maxFailures {
		b.open = true
	}
	b.mu.Unlock()
}

// Reset closes the breaker and resets the failure count.
func (b *Breaker) Reset() {
	b.mu.Lock()
	b.open = false
	b.failures = 0
	b.mu.Unlock()
}
