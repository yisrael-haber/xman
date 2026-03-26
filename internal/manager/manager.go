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
	mu      sync.RWMutex
	backend Backend
	jobs    *jobs.Manager
	ctx     context.Context
}

// New creates a Manager. The backend is nil until SetBackend is called.
func New(jm *jobs.Manager) *Manager {
	return &Manager{jobs: jm}
}

// SetContext is called by app.go on startup to provide the Wails runtime context.
func (m *Manager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

// SetBackend installs the active backend after a successful Connect.
func (m *Manager) SetBackend(b Backend) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backend = b
}

// Disconnect tears down the active backend and clears it.
func (m *Manager) Disconnect(ctx context.Context) error {
	m.mu.Lock()
	b := m.backend
	m.backend = nil
	m.mu.Unlock()

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
		DisplayName:  b.DisplayName(),
		GuestOps:     caps.GuestOps,
		Inventory:    caps.Inventory,
		ToolsInstall: caps.ToolsInstall,
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
