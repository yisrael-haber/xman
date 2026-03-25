package inventory

import (
	"context"

	"manosphere/internal/vcenter"
)

// Binding exposes inventory operations to the Wails frontend.
type Binding struct {
	service *Service
	ctx     context.Context
}

// NewBinding creates a Binding backed by the given vCenter session.
func NewBinding(s *vcenter.Session) *Binding {
	return &Binding{service: NewService(s)}
}

// SetContext is called by app.go on startup to provide the Wails runtime context.
func (b *Binding) SetContext(ctx context.Context) {
	b.ctx = ctx
}

// InventoryHosts returns all ESXi hosts.
func (b *Binding) InventoryHosts() ([]HostInfo, error) {
	return b.service.ListHosts(b.ctx)
}

// InventoryDatastores returns all datastores.
func (b *Binding) InventoryDatastores() ([]DatastoreInfo, error) {
	return b.service.ListDatastores(b.ctx)
}
