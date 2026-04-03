package manager

import (
	"context"
	"fmt"
	"time"

	"xman/internal/jobs"
)

// VMInfo is a serialisable summary of a virtual machine.
type VMNetworkAdapter struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	NetworkID   string   `json:"networkId"`
	Network     string   `json:"network"`
	NetworkType string   `json:"networkType"`
	MACAddress  string   `json:"macAddress"`
	Connected   bool     `json:"connected"`
	IPAddresses []string `json:"ipAddresses"`
}

type VMInfo struct {
	Ref             string             `json:"ref"`
	Name            string             `json:"name"`
	PathSegments    []string           `json:"pathSegments"`
	DisplayPath     string             `json:"displayPath"`
	PowerState      string             `json:"powerState"`
	ToolsStatus     string             `json:"toolsStatus"`
	GuestOpsReady   bool               `json:"guestOpsReady"`
	GuestOS         string             `json:"guestOS"`
	GuestHostname   string             `json:"guestHostname,omitempty"`
	IPAddress       string             `json:"ipAddress"`
	NumCPU          int32              `json:"numCPU"`
	MemoryMB        int32              `json:"memoryMB"`
	Firmware        string             `json:"firmware,omitempty"`
	HardwareVersion string             `json:"hardwareVersion,omitempty"`
	UUID            string             `json:"uuid,omitempty"`
	Notes           string             `json:"notes,omitempty"`
	VMXPath         string             `json:"vmxPath,omitempty"`
	HostName        string             `json:"hostName,omitempty"`
	DatastoreNames  []string           `json:"datastoreNames,omitempty"`
	NetworkAdapters []VMNetworkAdapter `json:"networkAdapters,omitempty"`
}

func (m *Manager) vmList() ([]VMInfo, error) {
	b, err := m.getBackend()
	if err != nil {
		return nil, err
	}
	return b.ListVMs(m.operationContext())
}

func (m *Manager) vmGet(vmRef string) (VMInfo, error) {
	b, err := m.getBackend()
	if err != nil {
		return VMInfo{}, err
	}
	return b.GetVM(m.operationContext(), vmRef)
}

func powerStatePollInterval(elapsed time.Duration) time.Duration {
	switch {
	case elapsed < 2*time.Second:
		return 250 * time.Millisecond
	case elapsed < 10*time.Second:
		return 500 * time.Millisecond
	default:
		return time.Second
	}
}

func powerStateProgressInterval(elapsed time.Duration) time.Duration {
	switch {
	case elapsed < 10*time.Second:
		return 2 * time.Second
	default:
		return 5 * time.Second
	}
}

func waitForPowerState(ctx context.Context, b Backend, emit jobs.EmitFn, vmRef, desiredState, action string) error {
	started := time.Now()
	deadline := started.Add(45 * time.Second)
	nextProgress := started

	for {
		info, err := b.GetVM(ctx, vmRef)
		if err != nil {
			return fmt.Errorf("verifying power state: %w", err)
		}
		if info.PowerState == desiredState {
			emit(100, fmt.Sprintf("%s complete", action))
			return nil
		}

		now := time.Now()
		elapsed := now.Sub(started)
		if !now.Before(nextProgress) {
			emit(70, fmt.Sprintf("Waiting for VM to report %s (current: %s)...", desiredState, info.PowerState))
			nextProgress = now.Add(powerStateProgressInterval(elapsed))
		}

		if now.After(deadline) {
			return fmt.Errorf("%s started, but the VM did not report %s within 45 seconds", action, desiredState)
		}

		timer := time.NewTimer(powerStatePollInterval(elapsed))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (m *Manager) vmPowerOn(vmRef string) string {
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

func (m *Manager) vmPowerOff(vmRef string) string {
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

func (m *Manager) vmReset(vmRef string) string {
	return m.submitJob("power", "Reset", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		emit(10, "Resetting...")
		return b.Reset(ctx, vmRef)
	})
}

func (m *Manager) vmSuspend(vmRef string) string {
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
