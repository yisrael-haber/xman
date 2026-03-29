package manager

import (
	"context"
	"fmt"
	"time"

	"xman/internal/jobs"
)

// VMInfo is a serialisable summary of a virtual machine.
type VMInfo struct {
	Ref          string   `json:"ref"`
	Name         string   `json:"name"`
	PathSegments []string `json:"pathSegments"`
	DisplayPath  string   `json:"displayPath"`
	PowerState   string   `json:"powerState"`
	ToolsStatus  string   `json:"toolsStatus"`
	GuestOS      string   `json:"guestOS"`
	IPAddress    string   `json:"ipAddress"`
	NumCPU       int32    `json:"numCPU"`
	MemoryMB     int32    `json:"memoryMB"`
}

func (m *Manager) VMList() ([]VMInfo, error) {
	b, err := m.getBackend()
	if err != nil {
		return nil, err
	}
	return b.ListVMs(m.ctx)
}

func (m *Manager) VMGet(vmRef string) (VMInfo, error) {
	b, err := m.getBackend()
	if err != nil {
		return VMInfo{}, err
	}
	return b.GetVM(m.ctx, vmRef)
}

func waitForPowerState(ctx context.Context, b Backend, emit jobs.EmitFn, vmRef, desiredState, action string) error {
	deadline := time.NewTimer(45 * time.Second)
	ticker := time.NewTicker(750 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()

	for {
		vms, err := b.ListVMs(ctx)
		if err != nil {
			return fmt.Errorf("verifying power state: %w", err)
		}

		for _, vm := range vms {
			if vm.Ref != vmRef {
				continue
			}
			if vm.PowerState == desiredState {
				emit(100, fmt.Sprintf("%s complete", action))
				return nil
			}
			emit(70, fmt.Sprintf("Waiting for VM to report %s (current: %s)...", desiredState, vm.PowerState))
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("%s started, but the VM did not report %s within 45 seconds", action, desiredState)
		case <-ticker.C:
		}
	}
}

func (m *Manager) VMPowerOn(vmRef string) string {
	return m.submitJob("power", "Power On", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		emit(10, "Powering on...")
		if err := b.PowerOn(ctx, vmRef); err != nil {
			return err
		}
		return waitForPowerState(ctx, b, emit, vmRef, "poweredOn", "Power on")
	})
}

func (m *Manager) VMPowerOff(vmRef string) string {
	return m.submitJob("power", "Power Off", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		emit(10, "Powering off...")
		if err := b.PowerOff(ctx, vmRef); err != nil {
			return err
		}
		return waitForPowerState(ctx, b, emit, vmRef, "poweredOff", "Power off")
	})
}

func (m *Manager) VMReset(vmRef string) string {
	return m.submitJob("power", "Reset", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		emit(10, "Resetting...")
		return b.Reset(ctx, vmRef)
	})
}

func (m *Manager) VMSuspend(vmRef string) string {
	return m.submitJob("power", "Suspend", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		emit(10, "Suspending...")
		if err := b.Suspend(ctx, vmRef); err != nil {
			return err
		}
		return waitForPowerState(ctx, b, emit, vmRef, "suspended", "Suspend")
	})
}
