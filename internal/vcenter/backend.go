package vcenter

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xman/internal/jobs"
	"xman/internal/manager"

	"github.com/vmware/govmomi/guest/toolbox"
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

func (b *Backend) BackendType() string { return "vcenter" }

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
	tools, err := b.newToolboxClient(ctx, req.VMRef, req.Username, req.Password)
	if err != nil {
		return err
	}

	f, err := os.Open(req.LocalPath)
	if err != nil {
		return fmt.Errorf("opening local file: %w", err)
	}
	defer f.Close()

	emit(10, "Copying file to guest...")
	var fileAttrs types.BaseGuestFileAttributes
	if manager.IsWindows(req.GuestOS) {
		fileAttrs = &types.GuestWindowsFileAttributes{}
	} else {
		fileAttrs = &types.GuestPosixFileAttributes{}
	}
	emit(20, fmt.Sprintf("Uploading %s...", filepath.Base(req.LocalPath)))
	if err := tools.Upload(ctx, f, req.GuestPath, gsoap.DefaultUpload, fileAttrs, true); err != nil {
		return fmt.Errorf("uploading file: %w", err)
	}

	emit(100, "Upload complete.")
	return nil
}

func (b *Backend) Download(ctx context.Context, emit jobs.EmitFn, req manager.DownloadRequest) error {
	tools, err := b.newToolboxClient(ctx, req.VMRef, req.Username, req.Password)
	if err != nil {
		return err
	}

	emit(10, "Copying file from guest...")
	src, _, err := tools.Download(ctx, req.GuestPath)
	if err != nil {
		return fmt.Errorf("downloading file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(req.LocalPath)
	if err != nil {
		return fmt.Errorf("creating local file: %w", err)
	}
	defer dst.Close()

	emit(20, fmt.Sprintf("Downloading %s...", filepath.Base(req.GuestPath)))
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("writing local file: %w", err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("closing local file: %w", err)
	}

	emit(100, "Download complete.")
	return nil
}

func (b *Backend) GuestRun(ctx context.Context, emit jobs.EmitFn, req manager.RunRequest) error {
	tools, err := b.newToolboxClient(ctx, req.VMRef, req.Username, req.Password)
	if err != nil {
		return err
	}

	spec, outPath := buildGuestRunSpec(req)

	emit(10, "Executing command...")

	pid, err := tools.ProcessManager.StartProgram(ctx, tools.Authentication, &spec)
	if err != nil {
		return fmt.Errorf("starting command: %w", err)
	}

	exitCode, err := waitForGuestProcess(ctx, emit, tools, pid)
	if err != nil {
		return err
	}

	emit(80, "Downloading output...")
	data, err := downloadGuestFile(ctx, tools, outPath)
	if err != nil {
		return err
	}

	output := normalizeGuestRunOutput(data)
	if exitCode != 0 {
		emit(95, fmt.Sprintf("%s\n\n[exit code: %d]", output, exitCode))
		emit(100, "Command finished with non-zero exit status.")
		return nil
	}
	emit(95, output)
	emit(100, "Command completed.")
	return nil
}

func (b *Backend) newToolboxClient(ctx context.Context, vmRef, username, password string) (*toolbox.Client, error) {
	client, err := b.session.Client()
	if err != nil {
		return nil, err
	}

	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: vmRef}
	auth := &types.NamePasswordAuthentication{Username: username, Password: password}
	tools, err := toolbox.NewClient(ctx, client.Client, ref, auth)
	if err != nil {
		return nil, fmt.Errorf("creating guest toolbox client: %w", err)
	}
	return tools, nil
}

func buildGuestRunSpec(req manager.RunRequest) (types.GuestProgramSpec, string) {
	outName := fmt.Sprintf("exec_out_%d.txt", time.Now().UnixNano())
	if manager.IsWindows(req.GuestOS) {
		outPath := `C:\Users\Public\` + outName
		return types.GuestProgramSpec{
			ProgramPath:      manager.WinPSExePath,
			Arguments:        manager.WinPSCmdArgs(req.Command, outPath),
			WorkingDirectory: `C:\Users\Public`,
		}, outPath
	}

	outPath := "/tmp/" + outName
	return types.GuestProgramSpec{
		ProgramPath:      "/bin/sh",
		Arguments:        manager.PosixCaptureArgs(req.Command, outPath),
		WorkingDirectory: "/tmp",
	}, outPath
}

func waitForGuestProcess(ctx context.Context, emit jobs.EmitFn, tools *toolbox.Client, pid int64) (int32, error) {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			terminateGuestProcess(tools, pid)
			return -1, ctx.Err()
		default:
		}

		procs, err := tools.ProcessManager.ListProcesses(ctx, tools.Authentication, []int64{pid})
		if err != nil {
			return -1, fmt.Errorf("checking process status: %w", err)
		}
		if len(procs) > 0 && procs[0].ExitCode != -1 {
			return procs[0].ExitCode, nil
		}
		if time.Now().After(deadline) {
			terminateGuestProcess(tools, pid)
			return -1, fmt.Errorf("timed out waiting for command to finish")
		}

		emit(50, "Waiting for command to finish...")
		time.Sleep(1 * time.Second)
	}
}

func downloadGuestFile(ctx context.Context, tools *toolbox.Client, guestPath string) ([]byte, error) {
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error

	for {
		src, _, err := tools.Download(ctx, guestPath)
		if err == nil {
			defer src.Close()
			data, readErr := io.ReadAll(src)
			if readErr == nil {
				_ = tools.FileManager.DeleteFile(ctx, tools.Authentication, guestPath)
				return data, nil
			}
			lastErr = fmt.Errorf("reading output: %w", readErr)
		} else {
			lastErr = fmt.Errorf("downloading output: %w", err)
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Now().After(deadline) {
			return nil, lastErr
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func normalizeGuestRunOutput(data []byte) string {
	output := strings.TrimSpace(string(data))
	if len(output) > 16*1024 {
		output = output[:16*1024] + "\n[output truncated]"
	}
	if output == "" {
		return "(no output)"
	}
	return output
}

func terminateGuestProcess(tools *toolbox.Client, pid int64) {
	killCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = tools.ProcessManager.TerminateProcess(killCtx, tools.Authentication, pid)
}

type toolsInstallState struct {
	guestOS     string
	toolsStatus types.VirtualMachineToolsStatus
	poweredOn   bool
	hasGuest    bool
}

func readToolsInstallState(ctx context.Context, vm *object.VirtualMachine) (toolsInstallState, error) {
	var obj mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"runtime.powerState", "guest.toolsStatus", "config.guestFullName"}, &obj); err != nil {
		return toolsInstallState{}, fmt.Errorf("reading VM properties: %w", err)
	}

	state := toolsInstallState{
		poweredOn: obj.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn,
		hasGuest:  obj.Guest != nil,
	}
	if obj.Config != nil {
		state.guestOS = obj.Config.GuestFullName
	}
	if obj.Guest != nil {
		state.toolsStatus = obj.Guest.ToolsStatus
	}

	return state, nil
}

func validateToolsInstallState(state toolsInstallState) error {
	if !state.poweredOn {
		return fmt.Errorf("VM must be powered on to install VMware Tools")
	}
	if state.guestOS != "" && !manager.IsWindows(state.guestOS) {
		return fmt.Errorf("bundled VMware Tools is not recommended for Linux/macOS guests; install open-vm-tools via the guest package manager instead")
	}
	return nil
}

func mountToolsInstaller(ctx context.Context, emit jobs.EmitFn, vm *object.VirtualMachine) error {
	emit(10, "Mounting VMware Tools installer...")
	if err := vm.MountToolsInstaller(ctx); err != nil {
		return fmt.Errorf("mounting tools installer: %w", err)
	}
	emit(100, "VMware Tools ISO mounted. Open the CD-ROM drive inside the guest and run setup64.exe (or setup.exe on 32-bit) to complete installation.")
	return nil
}

func upgradeTools(ctx context.Context, emit jobs.EmitFn, vm *object.VirtualMachine) error {
	emit(10, "Requesting VMware Tools upgrade from vSphere...")
	task, err := vm.UpgradeTools(ctx, "")
	if err != nil {
		emit(50, "Automatic upgrade unavailable — mounting VMware Tools installer...")
		if mountErr := mountToolsInstaller(ctx, emit, vm); mountErr != nil {
			return fmt.Errorf("upgrade tools: %w; mount installer fallback: %w", err, mountErr)
		}
		return nil
	}

	emit(50, "Installing VMware Tools, this may take a few minutes...")
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("VMware Tools installation task failed: %w", err)
	}
	emit(100, "VMware Tools installed successfully.")
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

func (b *Backend) InstallTools(ctx context.Context, emit jobs.EmitFn, vmRef string) error {
	vm, err := b.vmObject(ctx, vmRef)
	if err != nil {
		return err
	}

	state, err := readToolsInstallState(ctx, vm)
	if err != nil {
		return err
	}
	if err := validateToolsInstallState(state); err != nil {
		return err
	}

	if state.hasGuest && state.toolsStatus == types.VirtualMachineToolsStatusToolsOk {
		emit(100, "VMware Tools are already up to date.")
		return nil
	}

	// UpgradeTools requires a running guest agent. On a fresh VM with no tools
	// installed it would create a task that blocks indefinitely. Mount the ISO
	// directly and let the user run the installer.
	if !state.hasGuest || state.toolsStatus == types.VirtualMachineToolsStatusToolsNotInstalled {
		return mountToolsInstaller(ctx, emit, vm)
	}

	return upgradeTools(ctx, emit, vm)
}
