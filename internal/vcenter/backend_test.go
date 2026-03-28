package vcenter

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"testing"

	"xman/internal/manager"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	testVCUsername = "user"
	testVCPassword = "pass"
)

type taskInProgressVM struct {
	*simulator.VirtualMachine
}

func (vm *taskInProgressVM) PowerOffVMTask(ctx *simulator.Context, req *types.PowerOffVM_Task) soap.HasFault {
	task := simulator.CreateTask(req.This, "powerOff", func(*simulator.Task) (types.AnyType, types.BaseMethodFault) {
		return nil, &types.TaskInProgress{}
	})

	return &methods.PowerOffVM_TaskBody{
		Res: &types.PowerOffVM_TaskResponse{
			Returnval: task.Run(ctx),
		},
	}
}

func TestNewBackendMetadataAndDisconnect(t *testing.T) {
	backend, model := newTestBackend(t)

	if got := backend.BackendType(); got != "vcenter" {
		t.Fatalf("BackendType() = %q, want %q", got, "vcenter")
	}

	caps := backend.Capabilities()
	if !caps.GuestOps || !caps.Inventory || !caps.ToolsInstall {
		t.Fatalf("Capabilities() = %+v, want all advertised vCenter capabilities enabled", caps)
	}

	if got := backend.DisplayName(); !strings.Contains(got, model.Service.Listen.Host) || !strings.HasPrefix(got, "vCenter @ ") {
		t.Fatalf("DisplayName() = %q, want prefix %q and host %q", got, "vCenter @ ", model.Service.Listen.Host)
	}

	if err := backend.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}

	if _, err := backend.ListVMs(context.Background()); err == nil || !strings.Contains(err.Error(), "not connected to vCenter") {
		t.Fatalf("ListVMs() after disconnect error = %v, want not connected error", err)
	}
}

func TestNewBackendRejectsBadCredentials(t *testing.T) {
	model := newTestModel()
	if err := model.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	model.Service.Listen = &url.URL{Host: "127.0.0.1:0", User: url.UserPassword(testVCUsername, testVCPassword)}
	server := model.Service.NewServer()
	t.Cleanup(func() {
		server.Close()
		model.Remove()
	})

	sdkURL := *server.URL
	sdkURL.User = nil

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if _, err := NewBackend(ctx, sdkURL.String(), testVCUsername, "wrong-password", true); err == nil {
		t.Fatal("NewBackend() error = nil, want authentication failure")
	}
}

func TestListVMsAndGetVM(t *testing.T) {
	backend, model := newTestBackend(t)
	ctx := context.Background()

	vms, err := backend.ListVMs(ctx)
	if err != nil {
		t.Fatalf("ListVMs() error = %v", err)
	}

	counts := model.Count()
	if len(vms) != counts.Machine {
		t.Fatalf("ListVMs() returned %d VMs, want %d", len(vms), counts.Machine)
	}

	vm := vms[0]
	if vm.Ref == "" || vm.Name == "" {
		t.Fatalf("ListVMs() returned incomplete VM info: %+v", vm)
	}

	got, err := backend.GetVM(ctx, vm.Ref)
	if err != nil {
		t.Fatalf("GetVM() error = %v", err)
	}

	if got.Ref != vm.Ref || got.Name != vm.Name {
		t.Fatalf("GetVM() = %+v, want matching VM %+v", got, vm)
	}
	if got.NumCPU <= 0 || got.MemoryMB <= 0 {
		t.Fatalf("GetVM() returned empty hardware fields: %+v", got)
	}
}

func TestGetVMInvalidRefReturnsError(t *testing.T) {
	backend, _ := newTestBackend(t)

	_, err := backend.GetVM(context.Background(), "vm-does-not-exist")
	if err == nil {
		t.Fatal("GetVM() error = nil, want invalid ref error")
	}
}

func TestListVMsEmptyInventory(t *testing.T) {
	model := simulator.VPX()
	model.Datacenter = 0
	model.Cluster = 0
	model.ClusterHost = 0
	model.Host = 0
	model.Datastore = 0
	model.Machine = 0

	backend, _ := newBackendWithModel(t, model)

	vms, err := backend.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs() error = %v", err)
	}
	if len(vms) != 0 {
		t.Fatalf("ListVMs() len = %d, want %d", len(vms), 0)
	}
}

func TestPowerLifecycle(t *testing.T) {
	backend, _ := newTestBackend(t)
	ctx := context.Background()
	vm := firstVM(t, backend)

	if vm.PowerState != "poweredOn" {
		t.Fatalf("initial PowerState = %q, want %q", vm.PowerState, "poweredOn")
	}

	if err := backend.PowerOff(ctx, vm.Ref); err != nil {
		t.Fatalf("PowerOff() error = %v", err)
	}
	if got := getVM(t, backend, vm.Ref); got.PowerState != "poweredOff" {
		t.Fatalf("PowerOff() left VM in state %q, want %q", got.PowerState, "poweredOff")
	}

	if err := backend.PowerOn(ctx, vm.Ref); err != nil {
		t.Fatalf("PowerOn() error = %v", err)
	}
	if got := getVM(t, backend, vm.Ref); got.PowerState != "poweredOn" {
		t.Fatalf("PowerOn() left VM in state %q, want %q", got.PowerState, "poweredOn")
	}

	if err := backend.Reset(ctx, vm.Ref); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	if got := getVM(t, backend, vm.Ref); got.PowerState != "poweredOn" {
		t.Fatalf("Reset() left VM in state %q, want %q", got.PowerState, "poweredOn")
	}
}

func TestPowerOperationsInvalidRefReturnError(t *testing.T) {
	backend, _ := newTestBackend(t)
	ctx := context.Background()

	if err := backend.PowerOn(ctx, "vm-does-not-exist"); err == nil {
		t.Fatal("PowerOn() error = nil, want invalid ref error")
	}
	if err := backend.PowerOff(ctx, "vm-does-not-exist"); err == nil {
		t.Fatal("PowerOff() error = nil, want invalid ref error")
	}
	if err := backend.Reset(ctx, "vm-does-not-exist"); err == nil {
		t.Fatal("Reset() error = nil, want invalid ref error")
	}
	if err := backend.Suspend(ctx, "vm-does-not-exist"); err == nil {
		t.Fatal("Suspend() error = nil, want invalid ref error")
	}
}

func TestSnapshotsLifecycle(t *testing.T) {
	backend, _ := newTestBackend(t)
	ctx := context.Background()
	vm := firstVM(t, backend)

	req := manager.CreateSnapshotRequest{
		VMRef:       vm.Ref,
		Name:        "test-snap",
		Description: "created by vcsim test",
	}
	if err := backend.CreateSnapshot(ctx, noEmit, req); err != nil {
		t.Fatalf("CreateSnapshot() error = %v", err)
	}

	snaps, err := backend.ListSnapshots(ctx, vm.Ref)
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}

	snap, ok := snapshotByName(snaps, req.Name)
	if !ok {
		t.Fatalf("ListSnapshots() missing snapshot %q: %+v", req.Name, snaps)
	}
	if snap.Description != req.Description {
		t.Fatalf("snapshot Description = %q, want %q", snap.Description, req.Description)
	}
	if snap.Ref == "" {
		t.Fatalf("snapshot Ref is empty: %+v", snap)
	}

	if err := backend.RevertSnapshot(ctx, noEmit, snap.Ref); err != nil {
		t.Fatalf("RevertSnapshot() error = %v", err)
	}

	if err := backend.DeleteSnapshot(ctx, noEmit, snap.Ref, false); err != nil {
		t.Fatalf("DeleteSnapshot() error = %v", err)
	}

	snaps, err = backend.ListSnapshots(ctx, vm.Ref)
	if err != nil {
		t.Fatalf("ListSnapshots() after delete error = %v", err)
	}
	if _, ok := snapshotByName(snaps, req.Name); ok {
		t.Fatalf("snapshot %q still present after delete: %+v", req.Name, snaps)
	}
}

func TestListSnapshotsReturnsEmptyWhenVMHasNoSnapshots(t *testing.T) {
	backend, _ := newTestBackend(t)
	vm := firstVM(t, backend)

	snaps, err := backend.ListSnapshots(context.Background(), vm.Ref)
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}
	if len(snaps) != 0 {
		t.Fatalf("ListSnapshots() len = %d, want %d", len(snaps), 0)
	}
}

func TestRevertSnapshotInvalidRefReturnsError(t *testing.T) {
	backend, _ := newTestBackend(t)

	if err := backend.RevertSnapshot(context.Background(), noEmit, "snapshot-does-not-exist"); err == nil {
		t.Fatal("RevertSnapshot() error = nil, want invalid snapshot error")
	}
}

func TestDeleteSnapshotInvalidRefReturnsError(t *testing.T) {
	backend, _ := newTestBackend(t)

	if err := backend.DeleteSnapshot(context.Background(), noEmit, "snapshot-does-not-exist", false); err == nil {
		t.Fatal("DeleteSnapshot() error = nil, want invalid snapshot error")
	}
}

func TestInventoryLists(t *testing.T) {
	backend, model := newTestBackend(t)
	ctx := context.Background()
	counts := model.Count()

	hosts, err := backend.ListHosts(ctx)
	if err != nil {
		t.Fatalf("ListHosts() error = %v", err)
	}
	if len(hosts) != counts.Host {
		t.Fatalf("ListHosts() returned %d hosts, want %d", len(hosts), counts.Host)
	}
	for _, host := range hosts {
		if host.Ref == "" || host.Name == "" {
			t.Fatalf("ListHosts() returned incomplete host info: %+v", host)
		}
	}

	datastores, err := backend.ListDatastores(ctx)
	if err != nil {
		t.Fatalf("ListDatastores() error = %v", err)
	}
	if len(datastores) != counts.Datastore {
		t.Fatalf("ListDatastores() returned %d datastores, want %d", len(datastores), counts.Datastore)
	}
	for _, ds := range datastores {
		if ds.Ref == "" || ds.Name == "" {
			t.Fatalf("ListDatastores() returned incomplete datastore info: %+v", ds)
		}
	}

	networks, err := backend.ListNetworks(ctx)
	if err != nil {
		t.Fatalf("ListNetworks() error = %v", err)
	}
	if len(networks.Switches) == 0 {
		t.Fatal("ListNetworks() returned no switches")
	}

	var sawPortGroup bool
	for _, sw := range networks.Switches {
		if sw.Name == "" || sw.Type == "" {
			t.Fatalf("ListNetworks() returned incomplete switch info: %+v", sw)
		}
		if !sort.StringsAreSorted(sw.Hosts) {
			t.Fatalf("ListNetworks() returned unsorted switch hosts: %+v", sw.Hosts)
		}
		if !sort.StringsAreSorted(sw.Uplinks) {
			t.Fatalf("ListNetworks() returned unsorted switch uplinks: %+v", sw.Uplinks)
		}
		portGroupNames := make([]string, 0, len(sw.PortGroups))
		if len(sw.PortGroups) > 0 {
			sawPortGroup = true
		}
		for _, pg := range sw.PortGroups {
			portGroupNames = append(portGroupNames, pg.Name)
			if !sort.StringsAreSorted(pg.Hosts) {
				t.Fatalf("ListNetworks() returned unsorted port group hosts: %+v", pg.Hosts)
			}
		}
		if !sort.StringsAreSorted(portGroupNames) {
			t.Fatalf("ListNetworks() returned unsorted port groups: %+v", portGroupNames)
		}
	}
	if !sawPortGroup {
		t.Fatal("ListNetworks() returned switches without any port groups")
	}

	switchKeys := make([]string, 0, len(networks.Switches))
	for _, sw := range networks.Switches {
		switchKeys = append(switchKeys, sw.Type+":"+sw.Name)
	}
	if !sort.StringsAreSorted(switchKeys) {
		t.Fatalf("ListNetworks() returned unsorted switches: %+v", switchKeys)
	}
}

func TestInventoryEmptyInventoryReturnsEmptySlices(t *testing.T) {
	model := simulator.VPX()
	model.Datacenter = 0
	model.Cluster = 0
	model.ClusterHost = 0
	model.Host = 0
	model.Datastore = 0
	model.Machine = 0
	backend, _ := newBackendWithModel(t, model)

	hosts, err := backend.ListHosts(context.Background())
	if err != nil {
		t.Fatalf("ListHosts() error = %v", err)
	}
	if len(hosts) != 0 {
		t.Fatalf("ListHosts() len = %d, want %d", len(hosts), 0)
	}

	datastores, err := backend.ListDatastores(context.Background())
	if err != nil {
		t.Fatalf("ListDatastores() error = %v", err)
	}
	if len(datastores) != 0 {
		t.Fatalf("ListDatastores() len = %d, want %d", len(datastores), 0)
	}

	networks, err := backend.ListNetworks(context.Background())
	if err != nil {
		t.Fatalf("ListNetworks() error = %v", err)
	}
	if len(networks.Switches) != 0 {
		t.Fatalf("ListNetworks() switch count = %d, want %d", len(networks.Switches), 0)
	}
}

func TestPowerOffPropagatesTaskFailure(t *testing.T) {
	backend, model := newTestBackend(t)
	ctx := context.Background()

	vmObj := model.Map().Any("VirtualMachine").(*simulator.VirtualMachine)
	model.Map().Put(&taskInProgressVM{VirtualMachine: vmObj})

	if err := backend.PowerOff(ctx, vmObj.Reference().Value); err == nil {
		t.Fatal("PowerOff() error = nil, want task failure")
	}
}

func TestInstallToolsRejectsPoweredOffVM(t *testing.T) {
	backend, model := newTestBackend(t)
	vmObj := model.Map().Any("VirtualMachine").(*simulator.VirtualMachine)
	vmObj.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOff

	err := backend.InstallTools(context.Background(), noEmit, vmObj.Reference().Value)
	if err == nil || !strings.Contains(err.Error(), "must be powered on") {
		t.Fatalf("InstallTools() error = %v, want powered-on precondition error", err)
	}
}

func TestInstallToolsRejectsNonWindowsGuests(t *testing.T) {
	backend, model := newTestBackend(t)
	vmObj := model.Map().Any("VirtualMachine").(*simulator.VirtualMachine)
	vmObj.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOn
	vmObj.Config.GuestFullName = "Ubuntu Linux (64-bit)"

	err := backend.InstallTools(context.Background(), noEmit, vmObj.Reference().Value)
	if err == nil || !strings.Contains(err.Error(), "open-vm-tools") {
		t.Fatalf("InstallTools() error = %v, want open-vm-tools guidance", err)
	}
}

func TestInstallToolsReturnsSuccessWhenAlreadyUpToDate(t *testing.T) {
	backend, model := newTestBackend(t)
	vmObj := model.Map().Any("VirtualMachine").(*simulator.VirtualMachine)
	vmObj.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOn
	vmObj.Config.GuestFullName = "Microsoft Windows Server 2022"
	vmObj.Guest.ToolsStatus = types.VirtualMachineToolsStatusToolsOk

	var emitted []string
	err := backend.InstallTools(context.Background(), func(_ int, message string) {
		emitted = append(emitted, message)
	}, vmObj.Reference().Value)
	if err != nil {
		t.Fatalf("InstallTools() error = %v", err)
	}
	if len(emitted) == 0 || emitted[len(emitted)-1] != "VMware Tools are already up to date." {
		t.Fatalf("InstallTools() emitted %v, want already-up-to-date message", emitted)
	}
}

func newTestBackend(t *testing.T) (*Backend, *simulator.Model) {
	t.Helper()

	return newBackendWithModel(t, newTestModel())
}

func newTestModel() *simulator.Model {
	model := simulator.VPX()
	model.Datacenter = 1
	model.Cluster = 1
	model.ClusterHost = 1
	model.Host = 1
	model.Datastore = 1
	model.Machine = 1
	model.Portgroup = 1
	return model
}

func newBackendWithModel(t *testing.T, model *simulator.Model) (*Backend, *simulator.Model) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	if err := model.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	model.Service.Listen = &url.URL{Host: "127.0.0.1:0", User: url.UserPassword(testVCUsername, testVCPassword)}
	server := model.Service.NewServer()

	sdkURL := *server.URL
	sdkURL.User = nil

	backend, err := NewBackend(ctx, sdkURL.String(), testVCUsername, testVCPassword, true)
	if err != nil {
		server.Close()
		model.Remove()
		cancel()
		t.Fatalf("NewBackend() error = %v", err)
	}

	t.Cleanup(func() {
		_ = backend.Disconnect(context.Background())
		cancel()
		server.Close()
		model.Remove()
	})

	return backend, model
}

func firstVM(t *testing.T, backend *Backend) manager.VMInfo {
	t.Helper()

	vms, err := backend.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs() error = %v", err)
	}
	if len(vms) == 0 {
		t.Fatal("ListVMs() returned no VMs")
	}
	return vms[0]
}

func getVM(t *testing.T, backend *Backend, vmRef string) manager.VMInfo {
	t.Helper()

	vm, err := backend.GetVM(context.Background(), vmRef)
	if err != nil {
		t.Fatalf("GetVM() error = %v", err)
	}
	return vm
}

func snapshotByName(snaps []manager.SnapshotInfo, name string) (manager.SnapshotInfo, bool) {
	for _, snap := range snaps {
		if snap.Name == name {
			return snap, true
		}
	}
	return manager.SnapshotInfo{}, false
}

func noEmit(int, string) {}
