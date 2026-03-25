package vminfo

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"manosphere/internal/vcenter"
)

// VMInfo is a serialisable summary of a virtual machine.
type VMInfo struct {
	Ref         string `json:"ref"`         // ManagedObjectReference value (e.g. "vm-42")
	Name        string `json:"name"`
	PowerState  string `json:"powerState"`  // poweredOn | poweredOff | suspended
	ToolsStatus string `json:"toolsStatus"` // toolsOk | toolsOld | toolsNotInstalled | toolsNotRunning
	GuestOS     string `json:"guestOS"`
	IPAddress   string `json:"ipAddress"`
	NumCPU      int32  `json:"numCPU"`
	MemoryMB    int32  `json:"memoryMB"`
}

// Service provides VM information operations against vCenter.
type Service struct {
	session *vcenter.Session
}

// NewService creates a vminfo Service.
func NewService(s *vcenter.Session) *Service {
	return &Service{session: s}
}

// ListVMs returns all VMs visible to the connected vCenter user.
func (s *Service) ListVMs(ctx context.Context) ([]VMInfo, error) {
	client, err := s.session.Client()
	if err != nil {
		return nil, err
	}

	finder := find.NewFinder(client.Client, true)

	// Use the default datacenter
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing VMs: %w", err)
	}

	props := []string{
		"config.name",
		"config.guestFullName",
		"config.hardware.numCPU",
		"config.hardware.memoryMB",
		"runtime.powerState",
		"guest.toolsStatus",
		"guest.ipAddress",
	}

	out := make([]VMInfo, 0, len(vms))
	for _, vm := range vms {
		var obj mo.VirtualMachine
		if err := vm.Properties(ctx, vm.Reference(), props, &obj); err != nil {
			continue // skip VMs we can't read
		}
		out = append(out, toVMInfo(obj))
	}
	return out, nil
}

func toVMInfo(obj mo.VirtualMachine) VMInfo {
	info := VMInfo{
		Ref:        obj.Reference().Value,
		PowerState: string(obj.Runtime.PowerState),
	}
	if obj.Config != nil {
		info.Name = obj.Config.Name
		info.GuestOS = obj.Config.GuestFullName
		info.NumCPU = obj.Config.Hardware.NumCPU
		info.MemoryMB = obj.Config.Hardware.MemoryMB
	}
	if obj.Guest != nil {
		info.ToolsStatus = string(obj.Guest.ToolsStatus)
		info.IPAddress = obj.Guest.IpAddress
	}
	return info
}

// ToolsReady returns true if VMware Tools are installed and running in the VM.
func ToolsReady(status string) bool {
	return types.VirtualMachineToolsStatus(status) == types.VirtualMachineToolsStatusToolsOk ||
		types.VirtualMachineToolsStatus(status) == types.VirtualMachineToolsStatusToolsOld
}

func (s *Service) vmObject(ctx context.Context, vmRef string) (*object.VirtualMachine, error) {
	client, err := s.session.Client()
	if err != nil {
		return nil, err
	}
	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: vmRef}
	return object.NewVirtualMachine(client.Client, ref), nil
}

// PowerOn powers on a VM and waits for the task to complete.
func (s *Service) PowerOn(ctx context.Context, vmRef string) error {
	vm, err := s.vmObject(ctx, vmRef)
	if err != nil {
		return err
	}
	task, err := vm.PowerOn(ctx)
	if err != nil {
		return fmt.Errorf("power on: %w", err)
	}
	return task.Wait(ctx)
}

// PowerOff hard-powers off a VM and waits for the task to complete.
func (s *Service) PowerOff(ctx context.Context, vmRef string) error {
	vm, err := s.vmObject(ctx, vmRef)
	if err != nil {
		return err
	}
	task, err := vm.PowerOff(ctx)
	if err != nil {
		return fmt.Errorf("power off: %w", err)
	}
	return task.Wait(ctx)
}

// Reset resets a VM and waits for the task to complete.
func (s *Service) Reset(ctx context.Context, vmRef string) error {
	vm, err := s.vmObject(ctx, vmRef)
	if err != nil {
		return err
	}
	task, err := vm.Reset(ctx)
	if err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	return task.Wait(ctx)
}

// Suspend suspends a VM and waits for the task to complete.
func (s *Service) Suspend(ctx context.Context, vmRef string) error {
	vm, err := s.vmObject(ctx, vmRef)
	if err != nil {
		return err
	}
	task, err := vm.Suspend(ctx)
	if err != nil {
		return fmt.Errorf("suspend: %w", err)
	}
	return task.Wait(ctx)
}
