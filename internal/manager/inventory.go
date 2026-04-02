package manager

// HostInfo is a serialisable summary of an ESXi host.
type HostInfo struct {
	Ref             string `json:"ref"`
	Name            string `json:"name"`
	ConnectionState string `json:"connectionState"`
	PowerState      string `json:"powerState"`
	TotalCPUMHz     int32  `json:"totalCPUMHz"`
	UsedCPUMHz      int32  `json:"usedCPUMHz"`
	TotalMemoryMB   int64  `json:"totalMemoryMB"`
	UsedMemoryMB    int32  `json:"usedMemoryMB"`
	VMCount         int    `json:"vmCount"`
}

// DatastoreInfo is a serialisable summary of a datastore.
type DatastoreInfo struct {
	Ref        string  `json:"ref"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	CapacityGB float64 `json:"capacityGB"`
	FreeGB     float64 `json:"freeGB"`
	Accessible bool    `json:"accessible"`
}

func (m *Manager) InventoryHosts() ([]HostInfo, error) {
	b, err := m.getInventoryBackend()
	if err != nil {
		return nil, err
	}
	return b.ListHosts(m.operationContext())
}

func (m *Manager) InventoryDatastores() ([]DatastoreInfo, error) {
	b, err := m.getInventoryBackend()
	if err != nil {
		return nil, err
	}
	return b.ListDatastores(m.operationContext())
}
