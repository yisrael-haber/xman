package workstation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"xman/internal/jobs"
	"xman/internal/manager"
)

// Backend implements manager.Backend using the vmrun CLI.
// There is no persistent connection — vmrun is invoked per operation.
// Guest ops and snapshots are fully supported; host inventory is not
// (Workstation has no concept of ESXi hosts or datastores).
type Backend struct {
	vmrunPath string
	vmDir     string // custom VM directory; empty means use inventory.vmls + defaults
}

// NewBackend validates that vmrun is available and returns a ready Backend.
// If vmrunPath is empty, common install locations and PATH are tried.
// vmDir optionally overrides where VMs are searched; leave empty for defaults.
func NewBackend(vmrunPath, vmDir string) (*Backend, error) {
	if vmrunPath == "" {
		vmrunPath = detectVmrun()
	}

	// Verify the binary works
	cmd := exec.Command(vmrunPath, "list")
	configureCmd(cmd)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("vmrun not available at %q: %w", vmrunPath, err)
	}

	return &Backend{vmrunPath: vmrunPath, vmDir: vmDir}, nil
}

func (b *Backend) DisplayName() string { return "Local Workstation" }

func (b *Backend) Capabilities() manager.Capabilities {
	return manager.Capabilities{GuestOps: true, Inventory: false, ToolsInstall: true}
}

func (b *Backend) Disconnect(_ context.Context) error { return nil } // stateless

// --- vmrun helpers ---

// run executes vmrun with the given args and returns trimmed stdout.
func (b *Backend) run(args ...string) (string, error) {
	cmd := exec.Command(b.vmrunPath, args...)
	configureCmd(cmd)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if msg := strings.TrimSpace(string(exitErr.Stderr)); msg != "" {
				return "", fmt.Errorf("%s", msg)
			}
			if msg := strings.TrimSpace(string(out)); msg != "" {
				return "", fmt.Errorf("%s", msg)
			}
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ws prefixes args with the -T ws Workstation flag.
func ws(args ...string) []string {
	return append([]string{"-T", "ws"}, args...)
}

// guest prefixes args with -T ws and guest credentials.
func guest(user, pass string, args ...string) []string {
	return append([]string{"-T", "ws", "-gu", user, "-gp", pass}, args...)
}

// runningVMSet returns a set of .vmx paths that are currently powered on.
func (b *Backend) runningVMSet() (map[string]struct{}, error) {
	out, err := b.run("list")
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{})
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ".vmx") {
			set[line] = struct{}{}
		}
	}
	return set, nil
}

// --- VM lifecycle ---

func (b *Backend) ListVMs(_ context.Context) ([]manager.VMInfo, error) {
	invPath, err := inventoryPath()
	if err != nil {
		return nil, fmt.Errorf("locating inventory: %w", err)
	}

	var vmxPaths []string
	if b.vmDir != "" {
		vmxPaths, err = scanDirectory(b.vmDir)
	} else {
		vmxPaths, err = parseInventory(invPath)
		if err != nil {
			return nil, err
		}
		if len(vmxPaths) == 0 {
			vmxPaths, err = scanVMDirectories()
		}
	}
	if err != nil {
		return nil, err
	}

	running, err := b.runningVMSet()
	if err != nil {
		return nil, err
	}

	out := make([]manager.VMInfo, 0, len(vmxPaths))
	for _, vmx := range vmxPaths {
		info := manager.VMInfo{Ref: vmx}

		vmxData, err := parseVMX(vmx)
		if err == nil {
			info.Name = vmxData.DisplayName
			info.GuestOS = vmxData.GuestOS
			info.NumCPU = vmxData.NumCPU
			info.MemoryMB = vmxData.MemoryMB
		}

		if _, on := running[vmx]; on {
			info.PowerState = "poweredOn"
			info.ToolsStatus = b.checkToolsState(vmx)
			if info.ToolsStatus == "toolsOk" || info.ToolsStatus == "toolsOld" {
				if ip, err := b.run(ws("getGuestIPAddress", vmx)...); err == nil {
					info.IPAddress = strings.TrimSpace(ip)
				}
			}
		} else if isSuspended(vmx) {
			info.PowerState = "suspended"
			info.ToolsStatus = "toolsNotRunning"
		} else {
			info.PowerState = "poweredOff"
			info.ToolsStatus = "toolsNotRunning"
		}

		out = append(out, info)
	}
	return out, nil
}

// checkToolsState returns a toolsStatus string for a running VM.
func (b *Backend) checkToolsState(vmx string) string {
	out, err := b.run(ws("checkToolsState", vmx)...)
	if err != nil {
		return "toolsNotRunning"
	}
	if strings.EqualFold(out, "running") {
		return "toolsOk"
	}
	return "toolsNotRunning"
}

// isSuspended checks for a .vmss suspend-state file alongside the .vmx.
func isSuspended(vmx string) bool {
	dir := filepath.Dir(vmx)
	base := strings.TrimSuffix(filepath.Base(vmx), ".vmx")
	_, err := os.Stat(filepath.Join(dir, base+".vmss"))
	return err == nil
}

func (b *Backend) PowerOn(_ context.Context, vmRef string) error {
	_, err := b.run(ws("start", vmRef)...)
	return err
}

func (b *Backend) PowerOff(_ context.Context, vmRef string) error {
	_, err := b.run(ws("stop", vmRef, "hard")...)
	return err
}

func (b *Backend) Reset(_ context.Context, vmRef string) error {
	_, err := b.run(ws("reset", vmRef, "hard")...)
	return err
}

func (b *Backend) Suspend(_ context.Context, vmRef string) error {
	_, err := b.run(ws("suspend", vmRef)...)
	return err
}

// --- Snapshots ---
// vmrun identifies snapshots by name. Ref == Name for this backend.
// Tree depth and IsCurrent are not available from vmrun's output.

func (b *Backend) ListSnapshots(_ context.Context, vmRef string) ([]manager.SnapshotInfo, error) {
	out, err := b.run(ws("listSnapshots", vmRef)...)
	if err != nil {
		return nil, err
	}

	var snaps []manager.SnapshotInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		// Skip the "Total snapshots: N" header line
		if line == "" || strings.HasPrefix(line, "Total snapshots:") {
			continue
		}
		// Encode vmRef|snapName so Revert/Delete know which VM to operate on
		snaps = append(snaps, manager.SnapshotInfo{
			Ref:  vmRef + "|" + line,
			Name: line,
		})
	}
	return snaps, nil
}

func (b *Backend) CreateSnapshot(_ context.Context, emit jobs.EmitFn, req manager.CreateSnapshotRequest) error {
	emit(10, "Creating snapshot...")
	_, err := b.run(ws("snapshot", req.VMRef, req.Name)...)
	if err != nil {
		return err
	}
	emit(100, fmt.Sprintf("Snapshot %q created", req.Name))
	return nil
}

func (b *Backend) RevertSnapshot(_ context.Context, emit jobs.EmitFn, snapRef string) error {
	// snapRef is encoded as "vmRef\x00snapName" — see SnapshotRevert in manager
	vmRef, snapName := splitSnapRef(snapRef)
	emit(10, "Reverting to snapshot...")
	_, err := b.run(ws("revertToSnapshot", vmRef, snapName)...)
	if err != nil {
		return err
	}
	emit(100, "Reverted successfully")
	return nil
}

func (b *Backend) DeleteSnapshot(_ context.Context, emit jobs.EmitFn, snapRef string, _ bool) error {
	vmRef, snapName := splitSnapRef(snapRef)
	emit(10, "Deleting snapshot...")
	_, err := b.run(ws("deleteSnapshot", vmRef, snapName)...)
	if err != nil {
		return err
	}
	emit(100, "Deleted successfully")
	return nil
}

// splitSnapRef splits the compound "vmRef|snapName" ref used by the Workstation backend.
func splitSnapRef(ref string) (vmRef, snapName string) {
	idx := strings.LastIndex(ref, "|")
	if idx < 0 {
		return ref, ref
	}
	return ref[:idx], ref[idx+1:]
}

// --- Guest operations ---

// guestRunEnv returns the shell program, argument flag, and a temp output path
// appropriate for the VM's guest OS (detected from the .vmx file).
func guestRunEnv(vmx string) (prog, flag, outPath string) {
	outName := fmt.Sprintf("exec_out_%d.txt", time.Now().UnixNano())
	info, _ := parseVMX(vmx)
	if strings.Contains(strings.ToLower(info.GuestOS), "windows") {
		return `C:\Windows\System32\cmd.exe`, "/c", `C:\Users\Public\` + outName
	}
	return "/bin/sh", "-c", "/tmp/" + outName
}

func (b *Backend) Upload(_ context.Context, emit jobs.EmitFn, req manager.UploadRequest) error {
	emit(10, "Copying file to guest...")
	_, err := b.run(guest(req.Username, req.Password,
		"copyFileFromHostToGuest", req.VMRef, req.LocalPath, req.GuestPath)...)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	emit(100, "Upload complete.")
	return nil
}

func (b *Backend) Download(_ context.Context, emit jobs.EmitFn, req manager.DownloadRequest) error {
	emit(10, "Copying file from guest...")
	_, err := b.run(guest(req.Username, req.Password,
		"copyFileFromGuestToHost", req.VMRef, req.GuestPath, req.LocalPath)...)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	emit(100, "Download complete.")
	return nil
}

func (b *Backend) GuestRun(_ context.Context, emit jobs.EmitFn, req manager.RunRequest) error {
	prog, flag, outPath := guestRunEnv(req.VMRef)

	emit(10, "Executing command...")
	var runErr error
	if prog == `C:\Windows\System32\cmd.exe` {
		// cmd.exe I/O redirection doesn't work in VMware's headless guest-exec
		// session (no console attached). Use PowerShell with Out-File instead.
		// Pass flags as separate args — vmrun re-quotes a single string, which
		// makes PowerShell treat it as a positional parameter instead of flags.
		args := append(
			guest(req.Username, req.Password, "runProgramInGuest", req.VMRef, manager.WinPSExePath),
			manager.WinPSCmdArgList(req.Command, outPath)...,
		)
		emit(11, fmt.Sprintf("guest program: %s", manager.WinPSExePath))
		emit(12, fmt.Sprintf("outPath: %s", outPath))
		_, runErr = b.run(args...)
	} else {
		emit(11, fmt.Sprintf("guest program: %s %s", prog, flag))
		emit(12, fmt.Sprintf("outPath: %s", outPath))
		_, runErr = b.run(guest(req.Username, req.Password,
			"runProgramInGuest", req.VMRef,
			prog, flag, req.Command+" > "+outPath+" 2>&1")...)
	}
	emit(15, fmt.Sprintf("runProgramInGuest result: err=%v", runErr))

	// vmrun exits with code 1 whenever the guest program itself exits non-zero,
	// but the output file may still exist. Only bail out for genuine vmrun errors
	// (auth failures, tools not running, etc.) — not guest exit-code errors.
	if runErr != nil && !strings.Contains(runErr.Error(), "non-zero exit code") {
		return fmt.Errorf("running command: %w", runErr)
	}

	emit(80, "Downloading output...")
	tmpFile, err := os.CreateTemp("", "exec_out_*.txt")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	_, downloadErr := b.run(guest(req.Username, req.Password,
		"copyFileFromGuestToHost", req.VMRef, outPath, tmpPath)...)
	emit(85, fmt.Sprintf("copyFileFromGuestToHost result: err=%v", downloadErr))
	if downloadErr != nil {
		if runErr != nil {
			return fmt.Errorf("running command: %w\ndownload: %w", runErr, downloadErr)
		}
		return fmt.Errorf("downloading output: %w", downloadErr)
	}

	// best-effort cleanup of the temp file in the guest
	_, _ = b.run(guest(req.Username, req.Password,
		"deleteFileInGuest", req.VMRef, outPath)...)

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("reading output: %w", err)
	}

	output := strings.TrimSpace(string(data))
	if len(output) > 16*1024 {
		output = output[:16*1024] + "\n[output truncated]"
	}
	if output == "" {
		output = "(no output)"
	}
	if runErr != nil {
		emit(100, output+"\n\n["+runErr.Error()+"]")
		return nil
	}
	emit(100, output)
	return nil
}

// --- Inventory (unsupported) ---

func (b *Backend) ListHosts(_ context.Context) ([]manager.HostInfo, error) {
	return nil, fmt.Errorf("host inventory not available for Workstation")
}

func (b *Backend) ListDatastores(_ context.Context) ([]manager.DatastoreInfo, error) {
	return nil, fmt.Errorf("datastore inventory not available for Workstation")
}

func (b *Backend) InstallTools(ctx context.Context, emit jobs.EmitFn, vmRef string) error {
	info, err := parseVMX(vmRef)
	if err == nil && !strings.HasPrefix(strings.ToLower(info.GuestOS), "win") {
		return fmt.Errorf("bundled VMware Tools is not recommended for Linux/macOS guests; install open-vm-tools via the guest package manager instead")
	}

	emit(10, "Mounting VMware Tools installer...")

	// vmrun installTools blocks waiting for the guest agent to acknowledge. On a
	// fresh VM with no tools installed there is no agent, so it hangs indefinitely.
	// The hypervisor-level ISO mount happens immediately, so anything beyond ~30 s
	// is vmrun waiting for an agent that will never respond.
	mountCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(mountCtx, b.vmrunPath, "-T", "ws", "installTools", vmRef)
	configureCmd(cmd)
	_, cmdErr := cmd.Output()

	// Both a quick non-zero exit (no guest agent to respond) and a timeout (agent
	// never responded) mean the same thing: tools are not installed. In either case
	// the ISO is already mounted at the hypervisor level; guide the user.
	if cmdErr != nil {
		emit(100, "VMware Tools ISO mounted. Open the CD-ROM drive inside the guest and run setup64.exe (or setup.exe on 32-bit) to complete installation.")
		return nil
	}

	emit(100, "VMware Tools installation initiated in guest.")
	return nil
}
