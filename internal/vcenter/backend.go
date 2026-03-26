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

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/object"
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

	finder := find.NewFinder(client.Client, true)
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
		"config.name", "config.guestFullName",
		"config.hardware.numCPU", "config.hardware.memoryMB",
		"runtime.powerState", "guest.toolsStatus", "guest.ipAddress",
	}

	out := make([]manager.VMInfo, 0, len(vms))
	for _, vm := range vms {
		var obj mo.VirtualMachine
		if err := vm.Properties(ctx, vm.Reference(), props, &obj); err != nil {
			continue
		}
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

// isWindowsGuest returns true if the VM's guest OS is Windows.
func (b *Backend) isWindowsGuest(ctx context.Context, vmRef string) bool {
	vm, err := b.vmObject(ctx, vmRef)
	if err != nil {
		return false
	}
	var obj mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"config.guestId"}, &obj); err != nil {
		return false
	}
	return obj.Config != nil && strings.HasPrefix(strings.ToLower(obj.Config.GuestId), "win")
}

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
	if b.isWindowsGuest(ctx, req.VMRef) {
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
	if b.isWindowsGuest(ctx, req.VMRef) {
		outPath = `C:\Users\Public\` + outName
		spec = types.GuestProgramSpec{
			ProgramPath:      manager.WinPSExePath,
			Arguments:        manager.WinPSCmdArgs(req.Command, outPath),
			WorkingDirectory: `C:\Users\Public`,
		}
	} else {
		outPath = "/tmp/" + outName
		spec = types.GuestProgramSpec{
			ProgramPath:      "/bin/sh",
			Arguments:        fmt.Sprintf("-c '%s' > %s 2>&1", req.Command, outPath),
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
	emit(100, output)
	return nil
}

// --- Inventory ---

func (b *Backend) finder(ctx context.Context) (*find.Finder, error) {
	client, err := b.session.Client()
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

func (b *Backend) ListHosts(ctx context.Context) ([]manager.HostInfo, error) {
	f, err := b.finder(ctx)
	if err != nil {
		return nil, err
	}
	hosts, err := f.HostSystemList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing hosts: %w", err)
	}
	props := []string{
		"summary.config.name", "summary.runtime.connectionState",
		"summary.runtime.powerState", "summary.hardware.cpuMhz",
		"summary.hardware.numCpuCores", "summary.hardware.memorySize",
		"summary.quickStats.overallCpuUsage", "summary.quickStats.overallMemoryUsage",
		"vm",
	}
	out := make([]manager.HostInfo, 0, len(hosts))
	for _, h := range hosts {
		var obj mo.HostSystem
		if err := h.Properties(ctx, h.Reference(), props, &obj); err != nil {
			continue
		}
		s := obj.Summary
		info := manager.HostInfo{
			Ref:             h.Reference().Value,
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
	f, err := b.finder(ctx)
	if err != nil {
		return nil, err
	}
	datastores, err := f.DatastoreList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing datastores: %w", err)
	}
	props := []string{
		"summary.name", "summary.type",
		"summary.capacity", "summary.freeSpace", "summary.accessible",
	}
	const gb = 1024 * 1024 * 1024
	out := make([]manager.DatastoreInfo, 0, len(datastores))
	for _, ds := range datastores {
		var obj mo.Datastore
		if err := ds.Properties(ctx, ds.Reference(), props, &obj); err != nil {
			continue
		}
		info := manager.DatastoreInfo{Ref: ds.Reference().Value}
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
