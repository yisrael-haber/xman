package vminfo

import (
	"context"

	"manosphere/internal/vcenter"
)

// Binding exposes vminfo operations to the Wails frontend.
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

// VMList returns all VMs visible to the connected vCenter user.
func (b *Binding) VMList() ([]VMInfo, error) {
	return b.service.ListVMs(b.ctx)
}

// VMPowerOn powers on the specified VM.
func (b *Binding) VMPowerOn(vmRef string) error { return b.service.PowerOn(b.ctx, vmRef) }

// VMPowerOff hard-powers off the specified VM.
func (b *Binding) VMPowerOff(vmRef string) error { return b.service.PowerOff(b.ctx, vmRef) }

// VMReset resets the specified VM.
func (b *Binding) VMReset(vmRef string) error { return b.service.Reset(b.ctx, vmRef) }

// VMSuspend suspends the specified VM.
func (b *Binding) VMSuspend(vmRef string) error { return b.service.Suspend(b.ctx, vmRef) }
