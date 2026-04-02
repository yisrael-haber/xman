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
	"github.com/vmware/govmomi/vim25/mo"
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
	if !caps.GuestOps || !caps.Inventory || !caps.ToolsInstall || !caps.Console {
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

func TestConsoleURLBuildsVCenterHTML5ConsoleLink(t *testing.T) {
	backend, _ := newTestBackend(t)
	vm := firstVM(t, backend)

	info, err := backend.ConsoleInfo(context.Background(), vm.Ref)
	if err != nil {
		t.Fatalf("ConsoleInfo() error = %v", err)
	}

	u, err := url.Parse(info.URL)
	if err != nil {
		t.Fatalf("url.Parse(ConsoleInfo().URL) error = %v", err)
	}

	if u.User != nil {
		t.Fatalf("ConsoleInfo() leaked credentials in URL user info: %v", u.User)
	}
	if got := u.Path; got != "/ui/webconsole.html" {
		t.Fatalf("ConsoleInfo() path = %q, want %q", got, "/ui/webconsole.html")
	}

	q := u.Query()
	if got := q.Get("vmId"); got != vm.Ref {
		t.Fatalf("ConsoleInfo() vmId = %q, want %q", got, vm.Ref)
	}
	if got := q.Get("vmName"); got != vm.Name {
		t.Fatalf("ConsoleInfo() vmName = %q, want %q", got, vm.Name)
	}
	if got := q.Get("serverGuid"); got == "" {
		t.Fatal("ConsoleInfo() missing serverGuid")
	}
	if got := q.Get("sessionTicket"); got == "" {
		t.Fatal("ConsoleInfo() missing sessionTicket")
	}
	if _, ok := q["thumbprint"]; !ok {
		t.Fatal("ConsoleInfo() missing thumbprint query parameter")
	}
	if got := q.Get("host"); got == "" {
		t.Fatal("ConsoleInfo() missing host")
	}
}

func TestConsoleInfoPrefersReportedFQDNOverConnectedHost(t *testing.T) {
	backend := &Backend{}

	host, source, warnings, err := backend.consoleHost(&url.URL{Host: "10.20.30.40:443"}, "vcsa.internal.local")
	if err != nil {
		t.Fatalf("consoleHost() error = %v", err)
	}
	if host != "vcsa.internal.local" {
		t.Fatalf("consoleHost() host = %q, want %q", host, "vcsa.internal.local")
	}
	if source != consoleHostSourceReportedFQDN {
		t.Fatalf("consoleHost() source = %q, want %q", source, consoleHostSourceReportedFQDN)
	}
	if len(warnings) == 0 || !strings.Contains(warnings[0], "10.20.30.40") {
		t.Fatalf("consoleHost() warnings = %v, want mismatch warning mentioning connected host", warnings)
	}
}

func TestNormalizeInventoryPathSegmentsDropsDatacenterVMFolder(t *testing.T) {
	got := normalizeInventoryPathSegments("/Datacenter-A/vm/Team Folder/App")
	want := []string{"Datacenter-A", "Team Folder", "App"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("normalizeInventoryPathSegments() = %v, want %v", got, want)
	}
}

func TestToVMInfoPropagatesGuestOperationsReady(t *testing.T) {
	ready := true
	obj := mo.VirtualMachine{}
	obj.ManagedEntity.Self = types.ManagedObjectReference{Type: "VirtualMachine", Value: "vm-42"}
	obj.Config = &types.VirtualMachineConfigInfo{
		Name: "vm-42",
		Hardware: types.VirtualHardware{
			NumCPU:   2,
			MemoryMB: 4096,
		},
	}
	obj.Guest = &types.GuestInfo{
		ToolsStatus:          types.VirtualMachineToolsStatusToolsOk,
		GuestOperationsReady: &ready,
	}

	info := toVMInfo(obj, []string{"DC0", "Apps"}, nil)
	if !info.GuestOpsReady {
		t.Fatalf("toVMInfo() GuestOpsReady = %v, want true", info.GuestOpsReady)
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
	if len(vm.PathSegments) < 2 || vm.DisplayPath == "" {
		t.Fatalf("ListVMs() missing nested hierarchy info: %+v", vm)
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
	if len(got.PathSegments) < 2 || got.DisplayPath == "" {
		t.Fatalf("GetVM() missing nested display path: %+v", got)
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

func TestUpdateVMConfigAllowsRenameAndNotesWhilePoweredOn(t *testing.T) {
	backend, _ := newTestBackend(t)
	vm := firstVM(t, backend)

	req := manager.VMConfigUpdateRequest{
		VMRef:    vm.Ref,
		Name:     vm.Name + "-renamed",
		Notes:    "Primary application tier",
		NumCPU:   vm.NumCPU,
		MemoryMB: vm.MemoryMB,
		Firmware: vm.Firmware,
	}
	if err := backend.UpdateVMConfig(context.Background(), noEmit, req); err != nil {
		t.Fatalf("UpdateVMConfig() error = %v", err)
	}

	got := getVM(t, backend, vm.Ref)
	if got.PowerState != "poweredOn" {
		t.Fatalf("PowerState = %q, want %q", got.PowerState, "poweredOn")
	}
	if got.Name != req.Name {
		t.Fatalf("Name = %q, want %q", got.Name, req.Name)
	}
	if got.Notes != req.Notes {
		t.Fatalf("Notes = %q, want %q", got.Notes, req.Notes)
	}
}

func TestUpdateVMConfigRejectsHardwareChangesWhilePoweredOn(t *testing.T) {
	backend, _ := newTestBackend(t)
	vm := firstVM(t, backend)

	nextFirmware := "efi"
	if normalizeVCenterFirmware(vm.Firmware) == "efi" {
		nextFirmware = "bios"
	}

	err := backend.UpdateVMConfig(context.Background(), noEmit, manager.VMConfigUpdateRequest{
		VMRef:    vm.Ref,
		Name:     vm.Name,
		Notes:    vm.Notes,
		NumCPU:   vm.NumCPU + 1,
		MemoryMB: vm.MemoryMB + 1024,
		Firmware: nextFirmware,
	})
	if err == nil || !strings.Contains(err.Error(), "powered off") {
		t.Fatalf("UpdateVMConfig() error = %v, want powered-off precondition", err)
	}
}

func TestUpdateVMConfigAppliesHardwareChangesWhenPoweredOff(t *testing.T) {
	backend, _ := newTestBackend(t)
	vm := firstVM(t, backend)

	if err := backend.PowerOff(context.Background(), vm.Ref); err != nil {
		t.Fatalf("PowerOff() error = %v", err)
	}
	current := getVM(t, backend, vm.Ref)

	nextFirmware := "efi"
	if normalizeVCenterFirmware(current.Firmware) == "efi" {
		nextFirmware = "bios"
	}

	req := manager.VMConfigUpdateRequest{
		VMRef:    current.Ref,
		Name:     current.Name,
		Notes:    current.Notes,
		NumCPU:   current.NumCPU + 1,
		MemoryMB: current.MemoryMB + 1024,
		Firmware: nextFirmware,
	}
	if err := backend.UpdateVMConfig(context.Background(), noEmit, req); err != nil {
		t.Fatalf("UpdateVMConfig() error = %v", err)
	}

	got := getVM(t, backend, current.Ref)
	if got.PowerState != "poweredOff" {
		t.Fatalf("PowerState = %q, want %q", got.PowerState, "poweredOff")
	}
	if got.NumCPU != req.NumCPU {
		t.Fatalf("NumCPU = %d, want %d", got.NumCPU, req.NumCPU)
	}
	if got.MemoryMB != req.MemoryMB {
		t.Fatalf("MemoryMB = %d, want %d", got.MemoryMB, req.MemoryMB)
	}
	if normalizeVCenterFirmware(got.Firmware) != normalizeVCenterFirmware(req.Firmware) {
		t.Fatalf("Firmware = %q, want normalized %q", got.Firmware, req.Firmware)
	}
}

func TestListVMNetworkOptionsReturnsAttachableNetworks(t *testing.T) {
	model := newTestModel()
	model.Portgroup = 2

	backend, _ := newBackendWithModel(t, model)
	vm := firstVM(t, backend)

	options, err := backend.ListVMNetworkOptions(context.Background(), vm.Ref)
	if err != nil {
		t.Fatalf("ListVMNetworkOptions() error = %v", err)
	}
	if len(options) < 2 {
		t.Fatalf("ListVMNetworkOptions() len = %d, want at least %d (%+v)", len(options), 2, options)
	}
	for _, option := range options {
		if option.ID == "" || option.Name == "" || option.Type == "" {
			t.Fatalf("ListVMNetworkOptions() returned incomplete option: %+v", option)
		}
	}
}

func TestUpdateVMNetworkChangesNICAttachmentWhenPoweredOff(t *testing.T) {
	model := newTestModel()
	model.Portgroup = 2

	backend, _ := newBackendWithModel(t, model)
	vm := firstVM(t, backend)
	if err := backend.PowerOff(context.Background(), vm.Ref); err != nil {
		t.Fatalf("PowerOff() error = %v", err)
	}

	current := getVM(t, backend, vm.Ref)
	if len(current.NetworkAdapters) == 0 {
		t.Fatalf("GetVM() returned no network adapters: %+v", current)
	}
	adapter := current.NetworkAdapters[0]

	options, err := backend.ListVMNetworkOptions(context.Background(), vm.Ref)
	if err != nil {
		t.Fatalf("ListVMNetworkOptions() error = %v", err)
	}

	var target manager.VMNetworkOption
	for _, option := range options {
		if option.ID != adapter.NetworkID {
			target = option
			break
		}
	}
	if target.ID == "" {
		t.Fatalf("could not find alternate network option for adapter %+v from %+v", adapter, options)
	}

	if err := backend.UpdateVMNetwork(context.Background(), noEmit, manager.VMNetworkUpdateRequest{
		VMRef:     vm.Ref,
		AdapterID: adapter.ID,
		NetworkID: target.ID,
		Connected: false,
	}); err != nil {
		t.Fatalf("UpdateVMNetwork() error = %v", err)
	}

	got := getVM(t, backend, vm.Ref)
	if len(got.NetworkAdapters) == 0 {
		t.Fatalf("GetVM() after update returned no network adapters: %+v", got)
	}
	if got.NetworkAdapters[0].NetworkID != target.ID {
		t.Fatalf("NetworkID = %q, want %q", got.NetworkAdapters[0].NetworkID, target.ID)
	}
	if got.NetworkAdapters[0].Connected {
		t.Fatal("Connected = true, want false")
	}
	if got.NetworkAdapters[0].Network != target.Name {
		t.Fatalf("Network = %q, want %q", got.NetworkAdapters[0].Network, target.Name)
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

func TestValidateToolsInstallStateOnlyRequiresPoweredOn(t *testing.T) {
	if err := validateToolsInstallState(toolsInstallState{poweredOn: true}); err != nil {
		t.Fatalf("validateToolsInstallState(poweredOn) error = %v, want nil", err)
	}
}

func TestInstallToolsReturnsSuccessWhenAlreadyUpToDate(t *testing.T) {
	backend, model := newTestBackend(t)
	vmObj := model.Map().Any("VirtualMachine").(*simulator.VirtualMachine)
	vmObj.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOn
	vmObj.Config.GuestFullName = "Microsoft Windows Server 2022"
	vmObj.Guest.ToolsStatus = types.VirtualMachineToolsStatusToolsOk
	ready := true
	vmObj.Guest.GuestOperationsReady = &ready

	var emitted []string
	err := backend.InstallTools(context.Background(), func(_ int, message string) {
		emitted = append(emitted, message)
	}, vmObj.Reference().Value)
	if err != nil {
		t.Fatalf("InstallTools() error = %v", err)
	}
	if len(emitted) == 0 || emitted[len(emitted)-1] != "Guest operations are already ready." {
		t.Fatalf("InstallTools() emitted %v, want guest-ops-ready message", emitted)
	}
}

func TestInstallToolsReturnsWarmupMessageWhenToolsNeedTime(t *testing.T) {
	backend, model := newTestBackend(t)
	vmObj := model.Map().Any("VirtualMachine").(*simulator.VirtualMachine)
	vmObj.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOn
	vmObj.Config.GuestFullName = "Microsoft Windows Server 2022"
	vmObj.Guest.ToolsStatus = types.VirtualMachineToolsStatusToolsOk
	vmObj.Guest.GuestOperationsReady = nil

	var emitted []string
	err := backend.InstallTools(context.Background(), func(_ int, message string) {
		emitted = append(emitted, message)
	}, vmObj.Reference().Value)
	if err != nil {
		t.Fatalf("InstallTools() error = %v", err)
	}
	if len(emitted) == 0 || !strings.Contains(emitted[len(emitted)-1], "guest operations are still starting") {
		t.Fatalf("InstallTools() emitted %v, want guest-ops warmup message", emitted)
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
	model.Folder = 1
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
