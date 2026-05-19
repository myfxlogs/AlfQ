// Package risksvc — Kill Switch execution and alerts.
package risksvc

import (
	"encoding/json"
	"sync"
	"time"
)

// KillCommand is published to NATS risk.cmd when kill switch is activated.
type KillCommand struct {
	Action    string `json:"action"` // HALT, CANCEL_ALL, CLOSE_ALL, DISCONNECT
	Scope     string `json:"scope"`  // tenant_id or "global"
	ByUser    string `json:"by_user"`
	Reason    string `json:"reason"`
	Timestamp int64  `json:"ts_unix_ms"`
}

// KillExecutor handles the actual kill switch actions.
type KillExecutor struct {
	mu     sync.Mutex
	active bool
	cmds   []KillCommand
}

// NewKillExecutor creates a kill executor.
func NewKillExecutor() *KillExecutor { return &KillExecutor{} }

// Execute activates the kill switch and publishes commands.
// In production: publishes to NATS "risk.cmd" for all services to consume.
func (e *KillExecutor) Execute(scope, byUser, reason string) []KillCommand {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.active = true

	cmds := []KillCommand{
		{Action: "HALT", Scope: scope, ByUser: byUser, Reason: reason, Timestamp: time.Now().UnixMilli()},
		{Action: "CANCEL_ALL", Scope: scope, ByUser: byUser, Reason: reason, Timestamp: time.Now().UnixMilli()},
		{Action: "CLOSE_ALL", Scope: scope, ByUser: byUser, Reason: reason, Timestamp: time.Now().UnixMilli()},
		{Action: "DISCONNECT", Scope: scope, ByUser: byUser, Reason: reason, Timestamp: time.Now().UnixMilli()},
	}
	e.cmds = append(e.cmds, cmds...)
	return cmds
}

// Release deactivates the kill switch.
func (e *KillExecutor) Release() {
	e.mu.Lock()
	e.active = false
	e.mu.Unlock()
}

// IsActive returns whether the kill switch is engaged.
func (e *KillExecutor) IsActive() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.active
}

// Commands returns all issued kill commands.
func (e *KillExecutor) Commands() []KillCommand {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]KillCommand, len(e.cmds))
	copy(out, e.cmds)
	return out
}

// RiskEvent represents a risk-related audit event.
type RiskEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"` // rule_reject, kill_activated, kill_released, breaker_open, breaker_closed
	AccountID string `json:"account_id,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
	RuleID    string `json:"rule_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	ByUser    string `json:"by_user,omitempty"`
	Timestamp int64  `json:"ts_unix_ms"`
}

// EventRecorder records risk events for audit and alerting.
type EventRecorder struct {
	mu     sync.Mutex
	events []RiskEvent
}

// NewEventRecorder creates an event recorder.
func NewEventRecorder() *EventRecorder { return &EventRecorder{} }

// Record stores a risk event.
// In production: writes to PG risk_events table + publishes to NATS.
func (r *EventRecorder) Record(evt RiskEvent) {
	r.mu.Lock()
	r.events = append(r.events, evt)
	if len(r.events) > 1000 {
		r.events = r.events[len(r.events)-1000:]
	}
	r.mu.Unlock()
}

// Recent returns events from the last N seconds.
func (r *EventRecorder) Recent(maxAge time.Duration) []RiskEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	var out []RiskEvent
	for _, e := range r.events {
		if time.UnixMilli(e.Timestamp).After(cutoff) {
			out = append(out, e)
		}
	}
	return out
}

// MarshalCommands serializes kill commands for NATS publishing.
func MarshalCommands(cmds []KillCommand) ([]byte, error) {
	return json.Marshal(cmds)
}
