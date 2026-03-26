package manager

import "fmt"

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

func (m *Manager) VMPowerOn(vmRef string) error {
	b, err := m.getBackend()
	if err != nil {
		return err
	}
	return b.PowerOn(m.ctx, vmRef)
}

func (m *Manager) VMPowerOff(vmRef string) error {
	b, err := m.getBackend()
	if err != nil {
		return err
	}
	return b.PowerOff(m.ctx, vmRef)
}

func (m *Manager) VMReset(vmRef string) error {
	b, err := m.getBackend()
	if err != nil {
		return err
	}
	return b.Reset(m.ctx, vmRef)
}

func (m *Manager) VMSuspend(vmRef string) error {
	b, err := m.getBackend()
	if err != nil {
		return err
	}
	return b.Suspend(m.ctx, vmRef)
}

// toolsReady is a shared helper used by the frontend-facing feature tabs.
func toolsReady(status string) bool {
	return status == "toolsOk" || status == "toolsOld"
}

// ErrGuestOpsUnavailable is returned when the backend does not support guest operations.
var ErrGuestOpsUnavailable = fmt.Errorf("guest operations not supported by this backend")
