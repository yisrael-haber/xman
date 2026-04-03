package manager

import (
	"strconv"
)

// PortGroupInfo describes one port group (network) on a virtual switch.
type PortGroupInfo struct {
	Name    string   `json:"name"`
	VLAN    string   `json:"vlan"` // "—", "100", "trunk", "PVLAN 10"
	VMCount int      `json:"vmCount"`
	Hosts   []string `json:"hosts"`
}

// SwitchInfo describes a virtual switch (standard or distributed) and its port groups.
type SwitchInfo struct {
	Name       string          `json:"name"`
	Type       string          `json:"type"` // "standard" | "distributed"
	MTU        int32           `json:"mtu"`
	Uplinks    []string        `json:"uplinks"`
	Hosts      []string        `json:"hosts"`
	PortGroups []PortGroupInfo `json:"portGroups"`
}

// NetworkSummary is the top-level payload returned by ListNetworks.
type NetworkSummary struct {
	Switches []SwitchInfo `json:"switches"`
}

// FormatVLAN converts a vCenter VLAN ID integer to a display string.
func FormatVLAN(id int32) string {
	switch id {
	case 0:
		return "—"
	case 4095:
		return "trunk"
	default:
		return strconv.Itoa(int(id))
	}
}

// AppendUnique appends s to slice only if not already present.
func AppendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

func (m *Manager) inventoryNetworks() (NetworkSummary, error) {
	b, err := m.getBackend()
	if err != nil {
		return NetworkSummary{}, err
	}
	return b.ListNetworks(m.operationContext())
}
