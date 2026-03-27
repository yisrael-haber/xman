package manager

import (
	"context"

	"xman/internal/jobs"
)

// VMInfo is a serialisable summary of a virtual machine.
type VMInfo struct {
	Ref         string `json:"ref"`
	Name        string `json:"name"`
	PowerState  string `json:"powerState"`
	ToolsStatus string `json:"toolsStatus"`
	GuestOS     string `json:"guestOS"`
	IPAddress   string `json:"ipAddress"`
	NumCPU      int32  `json:"numCPU"`
	MemoryMB    int32  `json:"memoryMB"`
}

func (m *Manager) VMList() ([]VMInfo, error) {
	b, err := m.getBackend()
	if err != nil {
		return nil, err
	}
	return b.ListVMs(m.ctx)
}

func (m *Manager) VMPowerOn(vmRef string) string {
	return m.jobs.Submit("power", "Power On", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		emit(10, "Powering on...")
		return b.PowerOn(ctx, vmRef)
	})
}

func (m *Manager) VMPowerOff(vmRef string) string {
	return m.jobs.Submit("power", "Power Off", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		emit(10, "Powering off...")
		return b.PowerOff(ctx, vmRef)
	})
}

func (m *Manager) VMReset(vmRef string) string {
	return m.jobs.Submit("power", "Reset", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		emit(10, "Resetting...")
		return b.Reset(ctx, vmRef)
	})
}

func (m *Manager) VMSuspend(vmRef string) string {
	return m.jobs.Submit("power", "Suspend", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		emit(10, "Suspending...")
		return b.Suspend(ctx, vmRef)
	})
}
