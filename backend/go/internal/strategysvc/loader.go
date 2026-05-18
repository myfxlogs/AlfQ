// Package strategysvc — hot-reload support for strategy deployments.
package strategysvc

import (
	"context"
	"sync"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Loader manages strategy deployment lifecycle.
type Loader struct {
	mu      sync.RWMutex
	runners map[string]*Runner // deployment_id → runner
}

// NewLoader creates a deployment loader.
func NewLoader() *Loader {
	return &Loader{runners: make(map[string]*Runner)}
}

// Deploy creates and starts a new strategy runner.
func (l *Loader) Deploy(deploymentID string, strategy Strategy) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.runners[deploymentID] = NewRunner(strategy)
}

// Undeploy stops and removes a runner.
func (l *Loader) Undeploy(deploymentID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.runners, deploymentID)
}

// Get returns the runner for a deployment.
func (l *Loader) Get(deploymentID string) *Runner {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.runners[deploymentID]
}

// List returns all active deployment IDs.
func (l *Loader) List() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	ids := make([]string, 0, len(l.runners))
	for id := range l.runners {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of active runners.
func (l *Loader) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.runners)
}

// Status returns deployment status for monitoring.
type DeploymentStatus struct {
	ID         string
	Active     bool
	Positions  map[string]float64
}

// GetStatus returns the status of a deployment.
func (l *Loader) GetStatus(deploymentID string) *DeploymentStatus {
	l.mu.RLock()
	defer l.mu.RUnlock()
	r, ok := l.runners[deploymentID]
	if !ok {
		return &DeploymentStatus{ID: deploymentID, Active: false}
	}
	return &DeploymentStatus{
		ID:        deploymentID,
		Active:    true,
		Positions: r.position,
	}
}

// HotReload handles strategy parameter changes without full restart.
// In production: listens on PG NOTIFY or config change channel.
func (l *Loader) HotReload(ctx context.Context, deploymentID string, newStrategy Strategy) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if r, ok := l.runners[deploymentID]; ok {
		r.strategy = newStrategy // atomic swap of strategy instance
	}
}

// prevent unused import warnings
var _ = pb.OrderRequest{}
