package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/mo"
	"manosphere/internal/vcenter"
)

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

// Service provides host and datastore inventory operations.
type Service struct {
	session *vcenter.Session
}

// NewService creates an inventory Service.
func NewService(s *vcenter.Session) *Service {
	return &Service{session: s}
}

func (s *Service) finder(ctx context.Context) (*find.Finder, error) {
	client, err := s.session.Client()
	if err != nil {
		return nil, err
	}
	f := find.NewFinder(client.Client, true)
	dc, err := f.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding datacenter: %w", err)
	}
	f.SetDatacenter(dc)
	return f, nil
}

// ListHosts returns all ESXi hosts visible to the connected vCenter user.
func (s *Service) ListHosts(ctx context.Context) ([]HostInfo, error) {
	f, err := s.finder(ctx)
	if err != nil {
		return nil, err
	}

	hosts, err := f.HostSystemList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing hosts: %w", err)
	}

	props := []string{
		"summary.config.name",
		"summary.runtime.connectionState",
		"summary.runtime.powerState",
		"summary.hardware.cpuMhz",
		"summary.hardware.numCpuCores",
		"summary.hardware.memorySize",
		"summary.quickStats.overallCpuUsage",
		"summary.quickStats.overallMemoryUsage",
		"vm",
	}

	out := make([]HostInfo, 0, len(hosts))
	for _, h := range hosts {
		var obj mo.HostSystem
		if err := h.Properties(ctx, h.Reference(), props, &obj); err != nil {
			continue
		}
		s := obj.Summary
		info := HostInfo{
			Ref:             h.Reference().Value,
			VMCount:         len(obj.Vm),
			ConnectionState: string(s.Runtime.ConnectionState),
			PowerState:      string(s.Runtime.PowerState),
			UsedCPUMHz:      s.QuickStats.OverallCpuUsage,
			UsedMemoryMB:    s.QuickStats.OverallMemoryUsage,
		}
		info.Name = s.Config.Name
		if s.Hardware != nil {
			info.TotalCPUMHz = s.Hardware.CpuMhz * int32(s.Hardware.NumCpuCores)
			info.TotalMemoryMB = s.Hardware.MemorySize / (1024 * 1024)
		}
		out = append(out, info)
	}
	return out, nil
}

// ListDatastores returns all datastores visible to the connected vCenter user.
func (s *Service) ListDatastores(ctx context.Context) ([]DatastoreInfo, error) {
	f, err := s.finder(ctx)
	if err != nil {
		return nil, err
	}

	datastores, err := f.DatastoreList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing datastores: %w", err)
	}

	props := []string{
		"summary.name",
		"summary.type",
		"summary.capacity",
		"summary.freeSpace",
		"summary.accessible",
	}

	const gb = 1024 * 1024 * 1024
	out := make([]DatastoreInfo, 0, len(datastores))
	for _, ds := range datastores {
		var obj mo.Datastore
		if err := ds.Properties(ctx, ds.Reference(), props, &obj); err != nil {
			continue
		}
		info := DatastoreInfo{Ref: ds.Reference().Value}
		if s := obj.Summary; s.Name != "" {
			info.Name = s.Name
			info.Type = s.Type
			info.CapacityGB = float64(s.Capacity) / gb
			info.FreeGB = float64(s.FreeSpace) / gb
			info.Accessible = s.Accessible
		}
		out = append(out, info)
	}
	return out, nil
}
