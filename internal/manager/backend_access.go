package manager

import "fmt"

func (m *Manager) getGuestOpsBackend() (GuestOpsBackend, error) {
	b, err := m.getBackend()
	if err != nil {
		return nil, err
	}
	if !b.Capabilities().GuestOps {
		return nil, fmt.Errorf("guest operations are not supported by the active %s backend", b.BackendType())
	}
	gb, ok := b.(GuestOpsBackend)
	if !ok {
		return nil, fmt.Errorf("active %s backend advertises guest operations but does not implement them", b.BackendType())
	}
	return gb, nil
}

func (m *Manager) getInventoryBackend() (InventoryBackend, error) {
	b, err := m.getBackend()
	if err != nil {
		return nil, err
	}
	if !b.Capabilities().Inventory {
		return nil, fmt.Errorf("host inventory is not supported by the active %s backend", b.BackendType())
	}
	ib, ok := b.(InventoryBackend)
	if !ok {
		return nil, fmt.Errorf("active %s backend advertises inventory but does not implement it", b.BackendType())
	}
	return ib, nil
}

func (m *Manager) getToolsInstallBackend() (ToolsInstallBackend, error) {
	b, err := m.getBackend()
	if err != nil {
		return nil, err
	}
	if !b.Capabilities().ToolsInstall {
		return nil, fmt.Errorf("VMware Tools install is not supported by the active %s backend", b.BackendType())
	}
	tb, ok := b.(ToolsInstallBackend)
	if !ok {
		return nil, fmt.Errorf("active %s backend advertises tools install but does not implement it", b.BackendType())
	}
	return tb, nil
}

func (m *Manager) getConsoleBackend() (ConsoleBackend, error) {
	b, err := m.getBackend()
	if err != nil {
		return nil, err
	}
	if !b.Capabilities().Console {
		return nil, fmt.Errorf("web console is not supported by the active %s backend", b.BackendType())
	}
	cb, ok := b.(ConsoleBackend)
	if !ok {
		return nil, fmt.Errorf("active %s backend advertises console support but does not implement it", b.BackendType())
	}
	return cb, nil
}
