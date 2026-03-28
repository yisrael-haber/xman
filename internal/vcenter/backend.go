package vcenter

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xman/internal/jobs"
	"xman/internal/manager"

	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	gsoap "github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// Backend implements manager.Backend against a vCenter session.
type Backend struct {
	session *Session
}

// NewBackend connects to vCenter and returns a ready Backend.
func NewBackend(ctx context.Context, vcURL, username, password string, insecure bool) (*Backend, error) {
	s := &Session{}
	if err := s.Connect(ctx, ConnectParams{
		URL:      vcURL,
		Username: username,
		Password: password,
		Insecure: insecure,
	}); err != nil {
		return nil, err
	}
	s.StartKeepAlive(ctx)
	return &Backend{session: s}, nil
}

func (b *Backend) DisplayName() string {
	return "vCenter @ " + b.session.Host()
}

func (b *Backend) Capabilities() manager.Capabilities {
	return manager.Capabilities{GuestOps: true, Inventory: true, ToolsInstall: true}
}

func (b *Backend) Disconnect(ctx context.Context) error {
	return b.session.Disconnect(ctx)
}

// --- VM lifecycle ---

func (b *Backend) ListVMs(ctx context.Context) ([]manager.VMInfo, error) {
	client, err := b.session.Client()
	if err != nil {
		return nil, err
	}

	m := view.NewManager(client.Client)
	v, err := m.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("creating VM container view: %w", err)
	}
	defer v.Destroy(ctx)

	var vms []mo.VirtualMachine
	err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{
		"config.name", "config.guestFullName",
		"config.hardware.numCPU", "config.hardware.memoryMB",
		"runtime.powerState", "guest.toolsStatus", "guest.ipAddress",
	}, &vms)
	if err != nil {
		return nil, fmt.Errorf("listing VMs: %w", err)
	}

	out := make([]manager.VMInfo, 0, len(vms))
	for _, obj := range vms {
		out = append(out, toVMInfo(obj))
	}
	return out, nil
}

func toVMInfo(obj mo.VirtualMachine) manager.VMInfo {
	info := manager.VMInfo{
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

func (b *Backend) vmObject(ctx context.Context, vmRef string) (*object.VirtualMachine, error) {
	client, err := b.session.Client()
	if err != nil {
		return nil, err
	}
	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: vmRef}
	return object.NewVirtualMachine(client.Client, ref), nil
}

func (b *Backend) GetVM(ctx context.Context, vmRef string) (manager.VMInfo, error) {
	vm, err := b.vmObject(ctx, vmRef)
	if err != nil {
		return manager.VMInfo{}, err
	}

	var obj mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{
		"config.name", "config.guestFullName",
		"config.hardware.numCPU", "config.hardware.memoryMB",
		"runtime.powerState", "guest.toolsStatus", "guest.ipAddress",
	}, &obj); err != nil {
		return manager.VMInfo{}, fmt.Errorf("reading VM properties: %w", err)
	}

	return toVMInfo(obj), nil
}

func (b *Backend) PowerOn(ctx context.Context, vmRef string) error {
	vm, err := b.vmObject(ctx, vmRef)
	if err != nil {
		return err
	}
	task, err := vm.PowerOn(ctx)
	if err != nil {
		return fmt.Errorf("power on: %w", err)
	}
	return task.Wait(ctx)
}

func (b *Backend) PowerOff(ctx context.Context, vmRef string) error {
	vm, err := b.vmObject(ctx, vmRef)
	if err != nil {
		return err
	}
	task, err := vm.PowerOff(ctx)
	if err != nil {
		return fmt.Errorf("power off: %w", err)
	}
	return task.Wait(ctx)
}

func (b *Backend) Reset(ctx context.Context, vmRef string) error {
	vm, err := b.vmObject(ctx, vmRef)
	if err != nil {
		return err
	}
	task, err := vm.Reset(ctx)
	if err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	return task.Wait(ctx)
}

func (b *Backend) Suspend(ctx context.Context, vmRef string) error {
	vm, err := b.vmObject(ctx, vmRef)
	if err != nil {
		return err
	}
	task, err := vm.Suspend(ctx)
	if err != nil {
		return fmt.Errorf("suspend: %w", err)
	}
	return task.Wait(ctx)
}

// --- Snapshots ---

func (b *Backend) ListSnapshots(ctx context.Context, vmRef string) ([]manager.SnapshotInfo, error) {
	vm, err := b.vmObject(ctx, vmRef)
	if err != nil {
		return nil, err
	}

	var obj mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"snapshot"}, &obj); err != nil {
		return nil, fmt.Errorf("reading snapshot properties: %w", err)
	}
	if obj.Snapshot == nil {
		return []manager.SnapshotInfo{}, nil
	}

	var currentRef string
	if obj.Snapshot.CurrentSnapshot != nil {
		currentRef = obj.Snapshot.CurrentSnapshot.Value
	}

	var out []manager.SnapshotInfo
	var walk func(nodes []types.VirtualMachineSnapshotTree, depth int)
	walk = func(nodes []types.VirtualMachineSnapshotTree, depth int) {
		for _, node := range nodes {
			out = append(out, manager.SnapshotInfo{
				Ref:         node.Snapshot.Value,
				Name:        node.Name,
				Description: node.Description,
				CreateTime:  node.CreateTime,
				IsCurrent:   node.Snapshot.Value == currentRef,
				Depth:       depth,
			})
			walk(node.ChildSnapshotList, depth+1)
		}
	}
	walk(obj.Snapshot.RootSnapshotList, 0)
	return out, nil
}

func (b *Backend) CreateSnapshot(ctx context.Context, emit jobs.EmitFn, req manager.CreateSnapshotRequest) error {
	vm, err := b.vmObject(ctx, req.VMRef)
	if err != nil {
		return err
	}
	emit(10, "Creating snapshot...")
	task, err := vm.CreateSnapshot(ctx, req.Name, req.Description, req.Memory, req.Quiesce)
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("create snapshot task: %w", err)
	}
	emit(100, fmt.Sprintf("Snapshot %q created", req.Name))
	return nil
}

func (b *Backend) snapMOR(snapRef string) types.ManagedObjectReference {
	return types.ManagedObjectReference{Type: "VirtualMachineSnapshot", Value: snapRef}
}

func (b *Backend) RevertSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string) error {
	client, err := b.session.Client()
	if err != nil {
		return err
	}
	emit(10, "Reverting to snapshot...")
	res, err := methods.RevertToSnapshot_Task(ctx, client.Client, &types.RevertToSnapshot_Task{
		This: b.snapMOR(snapRef),
	})
	if err != nil {
		return fmt.Errorf("revert snapshot: %w", err)
	}
	if err := object.NewTask(client.Client, res.Returnval).Wait(ctx); err != nil {
		return fmt.Errorf("revert snapshot task: %w", err)
	}
	emit(100, "Reverted successfully")
	return nil
}

func (b *Backend) DeleteSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string, removeChildren bool) error {
	client, err := b.session.Client()
	if err != nil {
		return err
	}
	emit(10, "Deleting snapshot...")
	res, err := methods.RemoveSnapshot_Task(ctx, client.Client, &types.RemoveSnapshot_Task{
		This:           b.snapMOR(snapRef),
		RemoveChildren: removeChildren,
	})
	if err != nil {
		return fmt.Errorf("delete snapshot: %w", err)
	}
	if err := object.NewTask(client.Client, res.Returnval).Wait(ctx); err != nil {
		return fmt.Errorf("delete snapshot task: %w", err)
	}
	emit(100, "Deleted successfully")
	return nil
}

// --- Guest operations ---

func (b *Backend) Upload(ctx context.Context, emit jobs.EmitFn, req manager.UploadRequest) error {
	client, err := b.session.Client()
	if err != nil {
		return err
	}

	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: req.VMRef}
	ops := guest.NewOperationsManager(client.Client, ref)
	fm, err := ops.FileManager(ctx)
	if err != nil {
		return fmt.Errorf("getting guest file manager: %w", err)
	}

	guestAuth := &types.NamePasswordAuthentication{Username: req.Username, Password: req.Password}

	f, err := os.Open(req.LocalPath)
	if err != nil {
		return fmt.Errorf("opening local file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}

	emit(10, "Initiating transfer to guest...")
	var fileAttrs types.BaseGuestFileAttributes
	if manager.IsWindows(req.GuestOS) {
		fileAttrs = &types.GuestWindowsFileAttributes{}
	} else {
		fileAttrs = &types.GuestPosixFileAttributes{}
	}
	rawURL, err := fm.InitiateFileTransferToGuest(ctx, guestAuth, req.GuestPath,
		fileAttrs, fi.Size(), true)
	if err != nil {
		return fmt.Errorf("initiating upload: %w", err)
	}

	transferURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parsing transfer URL: %w", err)
	}

	emit(20, fmt.Sprintf("Uploading %s...", filepath.Base(req.LocalPath)))
	uploadParams := gsoap.DefaultUpload
	uploadParams.ContentLength = fi.Size()
	if err := client.Client.Upload(ctx, f, transferURL, &uploadParams); err != nil {
		return fmt.Errorf("uploading file: %w", err)
	}

	emit(100, "Upload complete.")
	return nil
}

func (b *Backend) Download(ctx context.Context, emit jobs.EmitFn, req manager.DownloadRequest) error {
	client, err := b.session.Client()
	if err != nil {
		return err
	}

	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: req.VMRef}
	ops := guest.NewOperationsManager(client.Client, ref)
	fm, err := ops.FileManager(ctx)
	if err != nil {
		return fmt.Errorf("getting guest file manager: %w", err)
	}

	guestAuth := &types.NamePasswordAuthentication{Username: req.Username, Password: req.Password}

	emit(10, "Requesting file info from guest...")
	fileInfo, err := fm.InitiateFileTransferFromGuest(ctx, guestAuth, req.GuestPath)
	if err != nil {
		return fmt.Errorf("initiating download: %w", err)
	}

	transferURL, err := url.Parse(fileInfo.Url)
	if err != nil {
		return fmt.Errorf("parsing transfer URL: %w", err)
	}

	emit(20, fmt.Sprintf("Downloading %s...", filepath.Base(req.GuestPath)))
	if err := client.Client.DownloadFile(ctx, req.LocalPath, transferURL, nil); err != nil {
		return fmt.Errorf("downloading file: %w", err)
	}

	emit(100, "Download complete.")
	return nil
}

func (b *Backend) GuestRun(ctx context.Context, emit jobs.EmitFn, req manager.RunRequest) error {
	client, err := b.session.Client()
	if err != nil {
		return err
	}

	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: req.VMRef}
	ops := guest.NewOperationsManager(client.Client, ref)
	auth := &types.NamePasswordAuthentication{Username: req.Username, Password: req.Password}

	pm, err := ops.ProcessManager(ctx)
	if err != nil {
		return fmt.Errorf("getting guest process manager: %w", err)
	}
	fm, err := ops.FileManager(ctx)
	if err != nil {
		return fmt.Errorf("getting guest file manager: %w", err)
	}

	outName := fmt.Sprintf("exec_out_%d.txt", time.Now().UnixNano())
	var outPath string
	var spec types.GuestProgramSpec
	if manager.IsWindows(req.GuestOS) {
		outPath = `C:\Users\Public\` + outName
		spec = types.GuestProgramSpec{
			ProgramPath:      manager.WinPSExePath,
			Arguments:        manager.WinPSCmdArgs(req.Command, outPath),
			WorkingDirectory: `C:\Users\Public`,
		}
	} else {
		outPath = "/tmp/" + outName
		// Escape single quotes in the command so the shell receives it as one
		// argument: ' → '\'' (end quote, literal single quote, reopen quote).
		escapedCmd := strings.ReplaceAll(req.Command, "'", `'\''`)
		spec = types.GuestProgramSpec{
			ProgramPath:      "/bin/sh",
			Arguments:        fmt.Sprintf("-c '%s > %s 2>&1'", escapedCmd, outPath),
			WorkingDirectory: "/tmp",
		}
	}

	emit(10, "Executing command...")

	pid, err := pm.StartProgram(ctx, auth, &spec)
	if err != nil {
		return fmt.Errorf("starting command: %w", err)
	}

	deadline := time.Now().Add(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			_ = pm.TerminateProcess(ctx, auth, pid)
			return ctx.Err()
		default:
		}
		procs, err := pm.ListProcesses(ctx, auth, []int64{pid})
		if err != nil {
			return fmt.Errorf("checking process status: %w", err)
		}
		if len(procs) > 0 && procs[0].ExitCode != -1 {
			break
		}
		if time.Now().After(deadline) {
			_ = pm.TerminateProcess(ctx, auth, pid)
			return fmt.Errorf("timed out waiting for command to finish")
		}
		emit(50, "Waiting for command to finish...")
		time.Sleep(1 * time.Second)
	}

	emit(80, "Downloading output...")
	fileInfo, err := fm.InitiateFileTransferFromGuest(ctx, auth, outPath)
	if err != nil {
		return fmt.Errorf("initiating output download: %w", err)
	}
	transferURL, err := url.Parse(fileInfo.Url)
	if err != nil {
		return fmt.Errorf("parsing transfer URL: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "exec_out_*.txt")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := client.Client.DownloadFile(ctx, tmpPath, transferURL, nil); err != nil {
		return fmt.Errorf("downloading output: %w", err)
	}
	_ = fm.DeleteFile(ctx, auth, outPath)

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("reading output: %w", err)
	}

	output := string(data)
	if len(output) > 16*1024 {
		output = output[:16*1024] + "\n[output truncated]"
	}
	if output == "" {
		output = "(no output)"
	}
	emit(95, output)
	emit(100, "Command completed.")
	return nil
}

// --- Inventory ---

func (b *Backend) ListHosts(ctx context.Context) ([]manager.HostInfo, error) {
	client, err := b.session.Client()
	if err != nil {
		return nil, err
	}

	m := view.NewManager(client.Client)
	v, err := m.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		return nil, fmt.Errorf("creating host container view: %w", err)
	}
	defer v.Destroy(ctx)

	var hosts []mo.HostSystem
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{
		"summary.config.name", "summary.runtime.connectionState",
		"summary.runtime.powerState", "summary.hardware.cpuMhz",
		"summary.hardware.numCpuCores", "summary.hardware.memorySize",
		"summary.quickStats.overallCpuUsage", "summary.quickStats.overallMemoryUsage",
		"vm",
	}, &hosts)
	if err != nil {
		return nil, fmt.Errorf("listing hosts: %w", err)
	}

	out := make([]manager.HostInfo, 0, len(hosts))
	for _, obj := range hosts {
		s := obj.Summary
		info := manager.HostInfo{
			Ref:             obj.Reference().Value,
			VMCount:         len(obj.Vm),
			ConnectionState: string(s.Runtime.ConnectionState),
			PowerState:      string(s.Runtime.PowerState),
			UsedCPUMHz:      s.QuickStats.OverallCpuUsage,
			UsedMemoryMB:    s.QuickStats.OverallMemoryUsage,
			Name:            s.Config.Name,
		}
		if s.Hardware != nil {
			info.TotalCPUMHz = s.Hardware.CpuMhz * int32(s.Hardware.NumCpuCores)
			info.TotalMemoryMB = s.Hardware.MemorySize / (1024 * 1024)
		}
		out = append(out, info)
	}
	return out, nil
}

func (b *Backend) ListDatastores(ctx context.Context) ([]manager.DatastoreInfo, error) {
	client, err := b.session.Client()
	if err != nil {
		return nil, err
	}

	m := view.NewManager(client.Client)
	v, err := m.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"Datastore"}, true)
	if err != nil {
		return nil, fmt.Errorf("creating datastore container view: %w", err)
	}
	defer v.Destroy(ctx)

	var datastores []mo.Datastore
	err = v.Retrieve(ctx, []string{"Datastore"}, []string{
		"summary.name", "summary.type",
		"summary.capacity", "summary.freeSpace", "summary.accessible",
	}, &datastores)
	if err != nil {
		return nil, fmt.Errorf("listing datastores: %w", err)
	}

	const gb = 1024 * 1024 * 1024
	out := make([]manager.DatastoreInfo, 0, len(datastores))
	for _, obj := range datastores {
		info := manager.DatastoreInfo{Ref: obj.Reference().Value}
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

func (b *Backend) ListNetworks(ctx context.Context) (manager.NetworkSummary, error) {
	client, err := b.session.Client()
	if err != nil {
		return manager.NetworkSummary{}, err
	}

	vm := view.NewManager(client.Client)

	// ── Step 1: All hosts → standard switches + standard port groups ──────────
	hv, err := vm.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		return manager.NetworkSummary{}, fmt.Errorf("host view: %w", err)
	}
	defer hv.Destroy(ctx)

	var hosts []mo.HostSystem
	if err := hv.Retrieve(ctx, []string{"HostSystem"}, []string{
		"name",
		"config.network.vswitch",
		"config.network.portgroup",
	}, &hosts); err != nil {
		return manager.NetworkSummary{}, fmt.Errorf("fetching hosts: %w", err)
	}

	// hostNames: MOR value → display name
	hostNames := make(map[string]string, len(hosts))
	for _, h := range hosts {
		hostNames[h.Reference().Value] = h.Name
	}

	// Accumulate standard switches: name → *SwitchInfo
	stdSW := make(map[string]*manager.SwitchInfo)
	// Accumulate standard PGs: switchName → pgName → *PortGroupInfo
	stdPG := make(map[string]map[string]*manager.PortGroupInfo)

	for _, h := range hosts {
		hName := hostNames[h.Reference().Value]
		if h.Config == nil || h.Config.Network == nil {
			continue
		}
		for _, vsw := range h.Config.Network.Vswitch {
			sw, ok := stdSW[vsw.Name]
			if !ok {
				sw = &manager.SwitchInfo{Name: vsw.Name, Type: "standard", MTU: vsw.Mtu}
				stdSW[vsw.Name] = sw
			}
			sw.Hosts = manager.AppendUnique(sw.Hosts, hName)
			if bridge, ok := vsw.Spec.Bridge.(*types.HostVirtualSwitchBondBridge); ok {
				for _, nic := range bridge.NicDevice {
					sw.Uplinks = manager.AppendUnique(sw.Uplinks, nic)
				}
			}
		}
		for _, pg := range h.Config.Network.Portgroup {
			swName := pg.Spec.VswitchName
			if stdPG[swName] == nil {
				stdPG[swName] = make(map[string]*manager.PortGroupInfo)
			}
			info, ok := stdPG[swName][pg.Spec.Name]
			if !ok {
				info = &manager.PortGroupInfo{
					Name: pg.Spec.Name,
					VLAN: manager.FormatVLAN(pg.Spec.VlanId),
				}
				stdPG[swName][pg.Spec.Name] = info
			}
			info.Hosts = manager.AppendUnique(info.Hosts, hName)
		}
	}

	// ── Step 2: Distributed virtual switches ──────────────────────────────────
	dv, err := vm.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"VmwareDistributedVirtualSwitch"}, true)
	if err != nil {
		return manager.NetworkSummary{}, fmt.Errorf("dvs view: %w", err)
	}
	defer dv.Destroy(ctx)

	var dvSwitches []mo.VmwareDistributedVirtualSwitch
	if err := dv.Retrieve(ctx, []string{"VmwareDistributedVirtualSwitch"}, []string{
		"name", "config",
	}, &dvSwitches); err != nil {
		return manager.NetworkSummary{}, fmt.Errorf("fetching dvs: %w", err)
	}

	// dvsSW: DVS MOR value → *SwitchInfo
	dvsSW := make(map[string]*manager.SwitchInfo, len(dvSwitches))

	for i := range dvSwitches {
		d := &dvSwitches[i]
		ref := d.Reference().Value
		sw := &manager.SwitchInfo{Name: d.Name, Type: "distributed"}

		if cfg, ok := d.Config.(*types.VMwareDVSConfigInfo); ok {
			sw.MTU = cfg.MaxMtu
			if pol, ok := cfg.UplinkPortPolicy.(*types.DVSNameArrayUplinkPortPolicy); ok {
				sw.Uplinks = pol.UplinkPortName
			}
		}
		// Host membership is derived from port groups below, after pg.Host is fetched.
		dvsSW[ref] = sw
	}

	// ── Step 3: DVS port groups ───────────────────────────────────────────────
	pgv, err := vm.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"DistributedVirtualPortgroup"}, true)
	if err != nil {
		return manager.NetworkSummary{}, fmt.Errorf("dvpg view: %w", err)
	}
	defer pgv.Destroy(ctx)

	var dvPGs []mo.DistributedVirtualPortgroup
	if err := pgv.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{
		"name", "config", "host", "vm",
	}, &dvPGs); err != nil {
		return manager.NetworkSummary{}, fmt.Errorf("fetching dvpgs: %w", err)
	}

	for _, pg := range dvPGs {
		if pg.Config.DistributedVirtualSwitch == nil {
			continue
		}
		swRef := pg.Config.DistributedVirtualSwitch.Value
		sw, ok := dvsSW[swRef]
		if !ok {
			continue
		}

		vlan := "—"
		if cfg, ok := pg.Config.DefaultPortConfig.(*types.VMwareDVSPortSetting); ok && cfg.Vlan != nil {
			switch v := cfg.Vlan.(type) {
			case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
				vlan = manager.FormatVLAN(v.VlanId)
			case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
				vlan = "trunk"
			case *types.VmwareDistributedVirtualSwitchPvlanSpec:
				vlan = fmt.Sprintf("PVLAN %d", v.PvlanId)
			}
		}

		pgInfo := manager.PortGroupInfo{
			Name:    pg.Name,
			VLAN:    vlan,
			VMCount: len(pg.Vm),
		}
		for _, href := range pg.Host {
			if name, ok := hostNames[href.Value]; ok {
				pgInfo.Hosts = manager.AppendUnique(pgInfo.Hosts, name)
			}
		}
		sw.PortGroups = append(sw.PortGroups, pgInfo)
	}

	// Derive DVS switch host lists from the union of their port groups' host membership.
	// This is more reliable than reading cfg.Host, which requires a type assertion
	// that can silently fail depending on how the config is deserialized.
	for _, sw := range dvsSW {
		for _, pg := range sw.PortGroups {
			for _, h := range pg.Hosts {
				sw.Hosts = manager.AppendUnique(sw.Hosts, h)
			}
		}
	}

	// ── Assemble result ───────────────────────────────────────────────────────
	var switches []manager.SwitchInfo

	// Standard switches (with their port groups)
	for _, sw := range stdSW {
		if pgs, ok := stdPG[sw.Name]; ok {
			for _, pg := range pgs {
				sw.PortGroups = append(sw.PortGroups, *pg)
			}
		}
		switches = append(switches, *sw)
	}
	// Distributed switches
	for _, sw := range dvsSW {
		switches = append(switches, *sw)
	}

	return manager.NetworkSummary{Switches: switches}, nil
}

func (b *Backend) InstallTools(ctx context.Context, emit jobs.EmitFn, vmRef string) error {
	vm, err := b.vmObject(ctx, vmRef)
	if err != nil {
		return err
	}

	var obj mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"runtime.powerState", "guest.toolsStatus"}, &obj); err != nil {
		return fmt.Errorf("reading VM properties: %w", err)
	}

	if obj.Runtime.PowerState != types.VirtualMachinePowerStatePoweredOn {
		return fmt.Errorf("VM must be powered on to install VMware Tools")
	}

	if obj.Guest != nil && obj.Guest.ToolsStatus == types.VirtualMachineToolsStatusToolsOk {
		emit(100, "VMware Tools are already up to date.")
		return nil
	}

	// UpgradeTools requires a running guest agent. On a fresh VM with no tools
	// installed it would create a task that blocks indefinitely. Mount the ISO
	// directly and let the user run the installer.
	if obj.Guest == nil || obj.Guest.ToolsStatus == types.VirtualMachineToolsStatusToolsNotInstalled {
		emit(10, "Mounting VMware Tools ISO...")
		if err := vm.MountToolsInstaller(ctx); err != nil {
			return fmt.Errorf("mounting tools installer: %w", err)
		}
		emit(100, "VMware Tools ISO mounted in the guest CD-ROM. Run the installer from within the guest to complete installation.")
		return nil
	}

	emit(10, "Requesting VMware Tools upgrade from vSphere...")
	task, err := vm.UpgradeTools(ctx, "")
	if err != nil {
		emit(50, "Automatic upgrade unavailable — mounting VMware Tools ISO...")
		if mountErr := vm.MountToolsInstaller(ctx); mountErr != nil {
			return fmt.Errorf("upgrade tools: %w; mount installer fallback: %w", err, mountErr)
		}
		emit(100, "VMware Tools ISO mounted in the guest CD-ROM. Run the installer from within the guest to complete installation.")
		return nil
	}

	emit(50, "Installing VMware Tools, this may take a few minutes...")
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("VMware Tools installation task failed: %w", err)
	}
	emit(100, "VMware Tools installed successfully.")
	return nil
}
