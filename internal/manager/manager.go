package manager

import (
	"context"
	"fmt"
	"sync"

	"xman/internal/config"
	"xman/internal/jobs"
)

// Manager is the single Wails-bound entry point for all VM feature operations.
// It holds one active Backend between Connect and Disconnect.
type Manager struct {
	mu         sync.RWMutex
	backend    Backend
	jobs       *jobs.Manager
	ctx        context.Context
	connCtx    context.Context
	connCancel context.CancelFunc
}

// New creates a Manager. The backend is nil until SetBackend is called.
func New(jm *jobs.Manager) *Manager {
	return &Manager{jobs: jm}
}

// SetContext is called by app.go on startup to provide the Wails runtime context.
func (m *Manager) SetContext(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ctx = ctx
}

// ReplaceBackend installs a new active backend, cancelling any in-flight
// connection-scoped jobs and disconnecting the previous backend first.
func (m *Manager) ReplaceBackend(ctx context.Context, b Backend) {
	m.mu.Lock()
	parentCtx := m.ctx
	oldBackend := m.backend
	oldCancel := m.connCancel

	if parentCtx == nil {
		parentCtx = context.Background()
	}
	connCtx, connCancel := context.WithCancel(parentCtx)

	m.backend = b
	m.connCtx = connCtx
	m.connCancel = connCancel
	m.mu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}
	if oldBackend != nil {
		_ = oldBackend.Disconnect(ctx)
	}
}

// Disconnect tears down the active backend and clears it.
func (m *Manager) Disconnect(ctx context.Context) error {
	m.mu.Lock()
	b := m.backend
	cancel := m.connCancel
	m.backend = nil
	m.connCtx = nil
	m.connCancel = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if b == nil {
		return nil
	}
	return b.Disconnect(ctx)
}

// ConnectionInfo returns display name and capabilities, or a zero value if not connected.
func (m *Manager) ConnectionInfo() config.ConnectionInfo {
	m.mu.RLock()
	b := m.backend
	m.mu.RUnlock()

	if b == nil {
		return config.ConnectionInfo{}
	}
	caps := b.Capabilities()
	return config.ConnectionInfo{
		BackendType:  b.BackendType(),
		DisplayName:  b.DisplayName(),
		GuestOps:     caps.GuestOps,
		Inventory:    caps.Inventory,
		ToolsInstall: caps.ToolsInstall,
		Console:      caps.Console,
	}
}

// getBackend returns the active backend or an error if not connected.
func (m *Manager) getBackend() (Backend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.backend == nil {
		return nil, fmt.Errorf("not connected")
	}
	return m.backend, nil
}

func (m *Manager) operationContext() context.Context {
	m.mu.RLock()
	defer m.mu.RUnlock()
	switch {
	case m.connCtx != nil:
		return m.connCtx
	case m.ctx != nil:
		return m.ctx
	default:
		return context.Background()
	}
}

func (m *Manager) submitJob(feature, label string, fn func(ctx context.Context, emit jobs.EmitFn) error) string {
	return m.jobs.SubmitWithParent(m.operationContext(), feature, label, fn)
}
