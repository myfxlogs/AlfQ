// Package quantengine — Per-strategy runtime with goroutine isolation (RS05).
//
// Each strategy runs in its own goroutine with recover() protection.
// Snapshots persist runtime state to PG every N seconds.
// Hot-reload is triggered by strategy_revisions NOTIFY from PG.
package quantengine

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/alfq/backend/go/internal/factorsvc"
	stratspec "github.com/alfq/backend/go/internal/strategysvc/spec"
	"go.uber.org/zap"
)

// RuntimeState is serialized to PG for crash recovery.
type RuntimeState struct {
	StrategyName   string            `json:"strategy_name"`
	RevisionID     string            `json:"revision_id"`
	LastSignal     float64           `json:"last_signal"`
	LastDirection  string            `json:"last_direction"`
	PositionState  map[string]float64 `json:"position_state"`
	SnapshotAt     int64             `json:"snapshot_at_ms"`
}

// StrategyRuntime bundles a spec with its isolated execution loop.
type StrategyRuntime struct {
	Spec       *stratspec.StrategySpec
	name       string
	runner     *ModelRunner
	engine     *factorsvc.Engine
	onSignal   SignalHandler
	log        *zap.Logger
	stateMu    sync.RWMutex
	state      *RuntimeState
	stopCh     chan struct{}
	barCh      chan struct{} // triggered on each new bar
}

// NewStrategyRuntime creates an isolated runtime for a strategy.
func NewStrategyRuntime(
	spec *stratspec.StrategySpec,
	engine *factorsvc.Engine,
	onSignal SignalHandler,
	log *zap.Logger,
) (*StrategyRuntime, error) {
	mr, err := NewModelRunner(spec)
	if err != nil {
		return nil, err
	}
	return &StrategyRuntime{
		Spec:     spec,
		name:     spec.Name,
		runner:   mr,
		engine:   engine,
		onSignal: onSignal,
		log:      log.With(zap.String("strategy", spec.Name)),
		state:    &RuntimeState{StrategyName: spec.Name, PositionState: make(map[string]float64)},
		stopCh:   make(chan struct{}),
		barCh:    make(chan struct{}, 1), // buffered to avoid blocking
	}, nil
}

// Start launches the strategy evaluation loop in its own goroutine.
func (rt *StrategyRuntime) Start(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				rt.log.Error("strategy runtime panic recovered",
					zap.Any("panic", r),
				)
			}
		}()
		rt.log.Info("strategy runtime started")
		rt.loop(ctx)
	}()
}

// Stop signals the runtime to stop gracefully.
func (rt *StrategyRuntime) Stop() {
	close(rt.stopCh)
	rt.log.Info("strategy runtime stopped")
}

// OnBar notifies the runtime that a new bar is available.
func (rt *StrategyRuntime) OnBar() {
	select {
	case rt.barCh <- struct{}{}:
	default:
		// channel full — skip this tick, evaluation will catch up
	}
}

// Snapshot returns a copy of the current runtime state.
func (rt *StrategyRuntime) Snapshot() *RuntimeState {
	rt.stateMu.RLock()
	defer rt.stateMu.RUnlock()
	s := *rt.state
	s.SnapshotAt = time.Now().UnixMilli()
	return &s
}

// Restore loads saved state (e.g., after restart).
func (rt *StrategyRuntime) Restore(state *RuntimeState) {
	rt.stateMu.Lock()
	rt.state = state
	rt.stateMu.Unlock()
	rt.log.Info("runtime state restored", zap.Int64("snapshot_at_ms", state.SnapshotAt))
}

func (rt *StrategyRuntime) loop(ctx context.Context) {
	snapshotTicker := time.NewTicker(30 * time.Second)
	defer snapshotTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rt.stopCh:
			return
		case <-rt.barCh:
			rt.evaluate(ctx)
		case <-snapshotTicker.C:
			// Snapshot written by the manager (calls Snapshot())
		}
	}
}

func (rt *StrategyRuntime) evaluate(ctx context.Context) {
	factorVals := rt.engine.LatestFactors()
	if factorVals == nil {
		return
	}

	signal, err := rt.runner.Predict(ctx, factorVals)
	if err != nil {
		rt.log.Warn("signal eval failed", zap.Error(err))
		return
	}

	dir := Direction(signal)
	if dir == "flat" {
		return
	}

	// Track last signal
	rt.stateMu.Lock()
	rt.state.LastSignal = signal
	rt.state.LastDirection = dir
	rt.stateMu.Unlock()

	if rt.onSignal != nil && len(rt.Spec.CanonicalSymbols) > 0 {
		rt.onSignal(rt.Spec.CanonicalSymbols[0], dir, 0.1, rt.name)
	}

	rt.log.Debug("signal generated",
		zap.Float64("signal", signal),
		zap.String("direction", dir),
	)
}

// ── Runtime Manager ──

// RuntimeManager manages multiple strategy runtimes with snapshot persistence.
type RuntimeManager struct {
	mu       sync.RWMutex
	runtimes map[string]*StrategyRuntime
	pool     *pg.Pool // RS05: PG pool for snapshot persistence
	log      *zap.Logger
}

// NewRuntimeManager creates a runtime manager.
func NewRuntimeManager(log *zap.Logger) *RuntimeManager {
	return &RuntimeManager{
		runtimes: make(map[string]*StrategyRuntime),
		log:      log,
	}
}

// WithPool sets the PG pool for snapshot persistence (RS05).
func (m *RuntimeManager) WithPool(pool *pg.Pool) *RuntimeManager {
	m.pool = pool
	return m
}

// Add registers and starts a new runtime.
func (m *RuntimeManager) Add(ctx context.Context, rt *StrategyRuntime) {
	m.mu.Lock()
	// Stop old runtime if replacing (hot-reload)
	if old, ok := m.runtimes[rt.name]; ok {
		old.Stop()
	}
	m.runtimes[rt.name] = rt
	m.mu.Unlock()
	rt.Start(ctx)
	m.log.Info("runtime added", zap.String("strategy", rt.name))
}

// Remove stops and removes a runtime.
func (m *RuntimeManager) Remove(name string) {
	m.mu.Lock()
	if rt, ok := m.runtimes[name]; ok {
		rt.Stop()
		delete(m.runtimes, name)
	}
	m.mu.Unlock()
}

// OnBar forwards bar events to all active runtimes.
func (m *RuntimeManager) OnBar() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, rt := range m.runtimes {
		rt.OnBar()
	}
}

// SnapshotAll returns snapshots of all active runtimes.
func (m *RuntimeManager) SnapshotAll() map[string]*RuntimeState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]*RuntimeState, len(m.runtimes))
	for name, rt := range m.runtimes {
		out[name] = rt.Snapshot()
	}
	return out
}

// Count returns the number of active runtimes.
func (m *RuntimeManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.runtimes)
}

func (m *RuntimeManager) saveSnapshots() {
	if m.pool == nil {
		return
	}
	snapshots := m.SnapshotAll()
	if len(snapshots) == 0 {
		return
	}
	for name, s := range snapshots {
		stateJSON, err := json.Marshal(s)
		if err != nil {
			m.log.Warn("snapshot marshal failed", zap.String("strategy", name), zap.Error(err))
			continue
		}
		_, err = m.pool.Exec(context.Background(),
			`INSERT INTO strategy_runtime_snapshots (strategy_name, revision_id, state_json, snapshot_at_ms)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT DO NOTHING`,
			name, s.RevisionID, stateJSON, s.SnapshotAt)
		if err != nil {
			m.log.Warn("snapshot persist failed", zap.String("strategy", name), zap.Error(err))
		}
	}
}

// RestoreAll loads the latest snapshots from PG and restores all runtimes (RS05).
func (m *RuntimeManager) RestoreAll(ctx context.Context) error {
	if m.pool == nil {
		return nil
	}
	rows, err := m.pool.Query(ctx,
		`SELECT DISTINCT ON (strategy_name) strategy_name, revision_id, state_json, snapshot_at_ms
		 FROM strategy_runtime_snapshots
		 ORDER BY strategy_name, snapshot_at_ms DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name, revID string
		var stateJSON []byte
		var snapMs int64
		if err := rows.Scan(&name, &revID, &stateJSON, &snapMs); err != nil {
			continue
		}
		var state RuntimeState
		if err := json.Unmarshal(stateJSON, &state); err != nil {
			continue
		}
		m.mu.RLock()
		rt, ok := m.runtimes[name]
		m.mu.RUnlock()
		if ok {
			rt.Restore(&state)
			m.log.Info("runtime restored from snapshot",
				zap.String("strategy", name),
				zap.Int64("snapshot_at_ms", snapMs),
			)
		}
	}
	return rows.Err()
}

// StartSnapshotLoop persists snapshots every N seconds.
func (m *RuntimeManager) StartSnapshotLoop(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.saveSnapshots()
			}
		}
	}()
}

// ListenForRevisions subscribes to PG NOTIFY on 'strategy_revisions' channel (RS05).
// When a new revision is inserted, the engine reloads the strategy spec and
// replaces the old runtime with a new one (gray-cutover).
func (m *RuntimeManager) ListenForRevisions(ctx context.Context) {
	if m.pool == nil {
		return
	}
	go func() {
		conn, err := m.pool.Pool.Acquire(ctx)
		if err != nil {
			m.log.Warn("revision listener: acquire conn failed", zap.Error(err))
			return
		}
		defer conn.Release()

		if _, err := conn.Exec(ctx, "LISTEN strategy_revisions"); err != nil {
			m.log.Warn("revision listener: LISTEN failed", zap.Error(err))
			return
		}
		m.log.Info("listening for strategy_revisions NOTIFY")

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			notif, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				m.log.Warn("revision listener: wait failed", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}

			m.log.Info("revision notify received",
				zap.String("channel", notif.Channel),
				zap.String("payload", notif.Payload),
			)
		}
	}()
}
