package manager

import (
	"context"
	"fmt"
	"strings"

	"xman/internal/jobs"
)

type VMConfigUpdateRequest struct {
	VMRef    string `json:"vmRef"`
	Name     string `json:"name"`
	Notes    string `json:"notes"`
	NumCPU   int32  `json:"numCPU"`
	MemoryMB int32  `json:"memoryMB"`
	Firmware string `json:"firmware"`
}

func validateVMConfigUpdateRequest(req VMConfigUpdateRequest) error {
	if strings.TrimSpace(req.VMRef) == "" {
		return fmt.Errorf("missing VM reference")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("VM name cannot be empty")
	}
	if req.NumCPU <= 0 {
		return fmt.Errorf("CPU count must be at least 1")
	}
	if req.MemoryMB <= 0 {
		return fmt.Errorf("memory must be greater than 0 MB")
	}
	switch normalized := strings.ToLower(strings.TrimSpace(req.Firmware)); normalized {
	case "", "bios", "efi", "uefi":
		return nil
	default:
		return fmt.Errorf("unsupported firmware value %q", req.Firmware)
	}
}

func (m *Manager) VMUpdateConfig(req VMConfigUpdateRequest) string {
	return m.submitJob("config", "Update VM Configuration", func(ctx context.Context, emit jobs.EmitFn) error {
		if err := validateVMConfigUpdateRequest(req); err != nil {
			return err
		}
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		return b.UpdateVMConfig(ctx, emit, req)
	})
}
