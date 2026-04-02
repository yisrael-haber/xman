package manager

import (
	"context"
	"fmt"
	"strings"

	"xman/internal/jobs"
)

type VMNetworkOption struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Group string `json:"group,omitempty"`
}

type VMNetworkUpdateRequest struct {
	VMRef     string `json:"vmRef"`
	AdapterID string `json:"adapterId"`
	NetworkID string `json:"networkId"`
	Connected bool   `json:"connected"`
}

func validateVMNetworkUpdateRequest(req VMNetworkUpdateRequest) error {
	if strings.TrimSpace(req.VMRef) == "" {
		return fmt.Errorf("missing VM reference")
	}
	if strings.TrimSpace(req.AdapterID) == "" {
		return fmt.Errorf("missing network adapter reference")
	}
	if strings.TrimSpace(req.NetworkID) == "" {
		return fmt.Errorf("missing target network")
	}
	return nil
}

func (m *Manager) VMNetworkOptions(vmRef string) ([]VMNetworkOption, error) {
	if strings.TrimSpace(vmRef) == "" {
		return nil, fmt.Errorf("missing VM reference")
	}
	b, err := m.getBackend()
	if err != nil {
		return nil, err
	}
	return b.ListVMNetworkOptions(m.operationContext(), vmRef)
}

func (m *Manager) VMUpdateNetwork(req VMNetworkUpdateRequest) string {
	return m.submitJob("network", "Update VM Network", func(ctx context.Context, emit jobs.EmitFn) error {
		if err := validateVMNetworkUpdateRequest(req); err != nil {
			return err
		}
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		return b.UpdateVMNetwork(ctx, emit, req)
	})
}
