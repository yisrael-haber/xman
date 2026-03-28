package manager

import (
	"context"

	"xman/internal/jobs"
)

func (m *Manager) VMInstallTools(vmRef string) string {
	return m.submitJob("tools", "Install VMware Tools", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		return b.InstallTools(ctx, emit, vmRef)
	})
}
