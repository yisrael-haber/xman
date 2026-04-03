package workstation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xman/internal/jobs"
	"xman/internal/manager"
)

func TestBackendListVMsWithFakeVmrun(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "ubuntu-dev", "ubuntu-64", 2, 4096)

	backend, _, _ := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT":       "Total running VMs: 1\n" + vmxPath + "\n",
		"FAKE_VMRUN_TOOLS_STATE":       "running",
		"FAKE_VMRUN_GUEST_IP":          "192.168.50.10",
		"FAKE_VMRUN_EXEC_OUTPUT":       "hello from guest\n",
		"FAKE_VMRUN_DOWNLOAD_CONTENT":  "downloaded file\n",
		"FAKE_VMRUN_SNAPSHOTS_OUTPUT":  "Total snapshots: 2\nsnap-a\nsnap-b\n",
		"FAKE_VMRUN_NONZERO_EXIT_TEXT": "non-zero exit code",
	})

	vms, err := backend.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs() error = %v", err)
	}
	if len(vms) != 1 {
		t.Fatalf("ListVMs() len = %d, want %d (%v)", len(vms), 1, vms)
	}

	vm := vms[0]
	if vm.Ref != vmxPath {
		t.Fatalf("Ref = %q, want %q", vm.Ref, vmxPath)
	}
	if vm.Name != "ubuntu-dev" || vm.GuestOS != "ubuntu-64" {
		t.Fatalf("ListVMs() returned wrong VM metadata: %+v", vm)
	}
	if vm.PowerState != "poweredOn" {
		t.Fatalf("PowerState = %q, want %q", vm.PowerState, "poweredOn")
	}
	if vm.ToolsStatus != "toolsOk" {
		t.Fatalf("ToolsStatus = %q, want %q", vm.ToolsStatus, "toolsOk")
	}
	if vm.IPAddress != "192.168.50.10" {
		t.Fatalf("IPAddress = %q, want %q", vm.IPAddress, "192.168.50.10")
	}
}

func TestBackendGetVMMissingReturnsError(t *testing.T) {
	backend, _, _ := newFakeVmrunBackend(t, t.TempDir(), map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT": "Total running VMs: 0\n",
	})

	_, err := backend.GetVM(context.Background(), "/tmp/does-not-exist.vmx")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("GetVM() error = %v, want not found", err)
	}
}

func TestListVMsAndGetVMReuseRunningVMSetCache(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "cache-vm", "ubuntu-64", 2, 4096)

	backend, _, logPath := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT": "Total running VMs: 1\n" + vmxPath + "\n",
		"FAKE_VMRUN_TOOLS_STATE": "running",
		"FAKE_VMRUN_GUEST_IP":    "192.168.50.20",
	})

	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", logPath, err)
	}

	if _, err := backend.ListVMs(context.Background()); err != nil {
		t.Fatalf("ListVMs() error = %v", err)
	}
	if _, err := backend.GetVM(context.Background(), vmxPath); err != nil {
		t.Fatalf("GetVM() error = %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logPath, err)
	}
	if got := countLogLines(string(logData), "list"); got != 1 {
		t.Fatalf("vmrun list count = %d, want %d\nfull log:\n%s", got, 1, string(logData))
	}
}

func TestBackendVMInfoFromPathUsesInventoryRootMetadataWithoutFilesystemFallback(t *testing.T) {
	backend := &Backend{vmDir: "/mnt/c/Users/yisra/RealDesktop/Virtual Machines"}

	info := backend.vmInfoFromPath(context.Background(), `C:\Users\yisra\RealDesktop\Virtual Machines\kali1\Debian 13.x 64-bit.vmx`, map[string]struct{}{}, false, &inventoryVM{
		DisplayName: "kali1",
	})

	if info.Name != "kali1" {
		t.Fatalf("Name = %q, want %q", info.Name, "kali1")
	}
	if len(info.PathSegments) != 0 {
		t.Fatalf("PathSegments = %v, want empty", info.PathSegments)
	}
	if info.DisplayPath != "" {
		t.Fatalf("DisplayPath = %q, want empty", info.DisplayPath)
	}
}

func TestBackendDoesNotImplementConsoleSupport(t *testing.T) {
	backend := &Backend{}

	if _, ok := any(backend).(manager.ConsoleBackend); ok {
		t.Fatal("Workstation backend unexpectedly implements manager.ConsoleBackend")
	}
}

func TestBackendPowerAndSnapshotCommands(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "ops-vm", "ubuntu-64", 2, 2048)

	backend, env, logPath := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT":      "Total running VMs: 0\n",
		"FAKE_VMRUN_SNAPSHOTS_OUTPUT": "Total snapshots: 2\nsnap-a\nsnap-b\n",
	})
	_ = env

	ctx := context.Background()
	if err := backend.PowerOn(ctx, vmxPath); err != nil {
		t.Fatalf("PowerOn() error = %v", err)
	}
	if err := backend.PowerOff(ctx, vmxPath); err != nil {
		t.Fatalf("PowerOff() error = %v", err)
	}
	if err := backend.Reset(ctx, vmxPath); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	if err := backend.Suspend(ctx, vmxPath); err != nil {
		t.Fatalf("Suspend() error = %v", err)
	}

	snaps, err := backend.ListSnapshots(ctx, vmxPath)
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("ListSnapshots() len = %d, want %d", len(snaps), 2)
	}
	if snaps[0].Ref != vmxPath+"|snap-a" {
		t.Fatalf("first snapshot Ref = %q, want %q", snaps[0].Ref, vmxPath+"|snap-a")
	}

	if err := backend.RevertSnapshot(ctx, noEmitWS, snaps[0].Ref); err != nil {
		t.Fatalf("RevertSnapshot() error = %v", err)
	}
	if err := backend.DeleteSnapshot(ctx, noEmitWS, snaps[1].Ref, false); err != nil {
		t.Fatalf("DeleteSnapshot() error = %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(log) error = %v", err)
	}
	logText := string(logData)
	for _, want := range []string{
		"list",
		"-T ws start " + vmxPath + " nogui",
		"-T ws stop " + vmxPath + " hard",
		"-T ws reset " + vmxPath + " hard",
		"-T ws suspend " + vmxPath,
		"-T ws listSnapshots " + vmxPath,
		"-T ws revertToSnapshot " + vmxPath + " snap-a",
		"-T ws deleteSnapshot " + vmxPath + " snap-b",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("fake vmrun log missing %q\nfull log:\n%s", want, logText)
		}
	}
}

func TestUpdateVMConfigPoweredOffRewritesVMX(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "config-vm", "ubuntu-64", 2, 2048)

	backend, _, _ := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT": "Total running VMs: 0\n",
	})

	req := manager.VMConfigUpdateRequest{
		VMRef:    vmxPath,
		Name:     "config-vm-renamed",
		Notes:    "Primary app\nNeeds maintenance window",
		NumCPU:   4,
		MemoryMB: 8192,
		Firmware: "efi",
	}
	if err := backend.UpdateVMConfig(context.Background(), noEmitWS, req); err != nil {
		t.Fatalf("UpdateVMConfig() error = %v", err)
	}

	got, err := backend.GetVM(context.Background(), vmxPath)
	if err != nil {
		t.Fatalf("GetVM() error = %v", err)
	}
	if got.Name != req.Name {
		t.Fatalf("Name = %q, want %q", got.Name, req.Name)
	}
	if got.Notes != req.Notes {
		t.Fatalf("Notes = %q, want %q", got.Notes, req.Notes)
	}
	if got.NumCPU != req.NumCPU {
		t.Fatalf("NumCPU = %d, want %d", got.NumCPU, req.NumCPU)
	}
	if got.MemoryMB != req.MemoryMB {
		t.Fatalf("MemoryMB = %d, want %d", got.MemoryMB, req.MemoryMB)
	}
	if got.Firmware != "UEFI" {
		t.Fatalf("Firmware = %q, want %q", got.Firmware, "UEFI")
	}

	raw, err := os.ReadFile(vmxPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", vmxPath, err)
	}
	text := string(raw)
	for _, want := range []string{
		`displayName = "config-vm-renamed"`,
		`annotation = "Primary app|0ANeeds maintenance window"`,
		`numvcpus = "4"`,
		`memsize = "8192"`,
		`firmware = "efi"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("updated VMX missing %q\nfull contents:\n%s", want, text)
		}
	}
}

func TestUpdateVMConfigRejectsRunningWorkstationVM(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "config-live", "ubuntu-64", 2, 2048)

	backend, _, logPath := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT": "Total running VMs: 1\n" + vmxPath + "\n",
	})

	err := backend.UpdateVMConfig(context.Background(), noEmitWS, manager.VMConfigUpdateRequest{
		VMRef:    vmxPath,
		Name:     "should-not-change",
		Notes:    "still live",
		NumCPU:   4,
		MemoryMB: 4096,
		Firmware: "efi",
	})
	if err == nil || !strings.Contains(err.Error(), "powered off") {
		t.Fatalf("UpdateVMConfig() error = %v, want powered-off precondition", err)
	}

	logData, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", logPath, readErr)
	}
	logText := string(logData)
	if strings.Contains(logText, "checkToolsState") || strings.Contains(logText, "getGuestIPAddress") {
		t.Fatalf("UpdateVMConfig() unexpectedly ran deep guest queries\nfull log:\n%s", logText)
	}
}

func TestUpdateVMNetworkPoweredOffRewritesVMX(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "network-vm", "ubuntu-64", 2, 2048)
	f, err := os.OpenFile(vmxPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile(%q) error = %v", vmxPath, err)
	}
	if _, err := f.WriteString("ethernet0.present = \"true\"\n" +
		"ethernet0.connectionType = \"nat\"\n" +
		"ethernet0.generatedAddress = \"00:50:56:aa:bb:cc\"\n" +
		"ethernet0.startConnected = \"true\"\n"); err != nil {
		_ = f.Close()
		t.Fatalf("WriteString(%q) error = %v", vmxPath, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close(%q) error = %v", vmxPath, err)
	}

	backend, _, _ := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT": "Total running VMs: 0\n",
	})

	if err := backend.UpdateVMNetwork(context.Background(), noEmitWS, manager.VMNetworkUpdateRequest{
		VMRef:     vmxPath,
		AdapterID: "ethernet0",
		NetworkID: "bridged",
		Connected: false,
	}); err != nil {
		t.Fatalf("UpdateVMNetwork() error = %v", err)
	}

	got, err := backend.GetVM(context.Background(), vmxPath)
	if err != nil {
		t.Fatalf("GetVM() error = %v", err)
	}
	if len(got.NetworkAdapters) != 1 {
		t.Fatalf("NetworkAdapters len = %d, want %d (%v)", len(got.NetworkAdapters), 1, got.NetworkAdapters)
	}
	if got.NetworkAdapters[0].NetworkID != "bridged" {
		t.Fatalf("NetworkID = %q, want %q", got.NetworkAdapters[0].NetworkID, "bridged")
	}
	if got.NetworkAdapters[0].Network != "Bridged (VMnet0)" {
		t.Fatalf("Network = %q, want %q", got.NetworkAdapters[0].Network, "Bridged (VMnet0)")
	}
	if got.NetworkAdapters[0].Connected {
		t.Fatal("Connected = true, want false")
	}

	raw, err := os.ReadFile(vmxPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", vmxPath, err)
	}
	text := string(raw)
	for _, want := range []string{
		`ethernet0.connectionType = "bridged"`,
		`ethernet0.startConnected = "false"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("updated VMX missing %q\nfull contents:\n%s", want, text)
		}
	}
	if strings.Contains(text, `ethernet0.vnet =`) {
		t.Fatalf("updated VMX unexpectedly kept ethernet0.vnet\nfull contents:\n%s", text)
	}
}

func TestUpdateVMNetworkRejectsRunningWorkstationVM(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "network-live", "ubuntu-64", 2, 2048)
	f, err := os.OpenFile(vmxPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile(%q) error = %v", vmxPath, err)
	}
	if _, err := f.WriteString("ethernet0.present = \"true\"\n" +
		"ethernet0.connectionType = \"nat\"\n"); err != nil {
		_ = f.Close()
		t.Fatalf("WriteString(%q) error = %v", vmxPath, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close(%q) error = %v", vmxPath, err)
	}

	backend, _, _ := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT": "Total running VMs: 1\n" + vmxPath + "\n",
	})

	err = backend.UpdateVMNetwork(context.Background(), noEmitWS, manager.VMNetworkUpdateRequest{
		VMRef:     vmxPath,
		AdapterID: "ethernet0",
		NetworkID: "bridged",
		Connected: true,
	})
	if err == nil || !strings.Contains(err.Error(), "powered off") {
		t.Fatalf("UpdateVMNetwork() error = %v, want powered-off precondition", err)
	}
}

func TestBackendGuestRunNonZeroReturnsNilAndEmitsOutput(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "exec-vm", "ubuntu-64", 2, 2048)

	backend, _, _ := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT":       "Total running VMs: 1\n" + vmxPath + "\n",
		"FAKE_VMRUN_EXEC_OUTPUT":       "line 1\nline 2\n",
		"FAKE_VMRUN_NONZERO_EXIT":      "1",
		"FAKE_VMRUN_NONZERO_EXIT_TEXT": "non-zero exit code",
	})

	var emitted []jobs.LogEntry
	emit := func(progress int, message string) {
		emitted = append(emitted, jobs.LogEntry{Progress: progress, Message: message})
	}

	err := backend.GuestRun(context.Background(), emit, manager.RunRequest{
		VMRef:    vmxPath,
		GuestOS:  "ubuntu-64",
		Username: "tester",
		Password: "secret",
		Command:  "echo hello && false",
	})
	if err != nil {
		t.Fatalf("GuestRun() error = %v, want nil on guest non-zero exit", err)
	}

	if len(emitted) == 0 {
		t.Fatal("GuestRun() emitted no progress messages")
	}
	last := emitted[len(emitted)-1]
	if last.Message != "Command finished with non-zero exit status." {
		t.Fatalf("final message = %q, want %q", last.Message, "Command finished with non-zero exit status.")
	}

	var sawOutput bool
	for _, entry := range emitted {
		if entry.Progress == 95 && strings.Contains(entry.Message, "line 1") && strings.Contains(entry.Message, "non-zero exit code") {
			sawOutput = true
			break
		}
	}
	if !sawOutput {
		t.Fatalf("GuestRun() did not emit detailed non-zero output: %+v", emitted)
	}
}

func TestBackendGuestRunReturnsBeforeCleanupFinishes(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "exec-fast", "ubuntu-64", 2, 2048)

	backend, _, _ := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT":        "Total running VMs: 1\n" + vmxPath + "\n",
		"FAKE_VMRUN_EXEC_OUTPUT":        "fast output\n",
		"FAKE_VMRUN_CLEANUP_SLEEP_SECS": "0.9",
	})

	started := time.Now()
	err := backend.GuestRun(context.Background(), noEmitWS, manager.RunRequest{
		VMRef:    vmxPath,
		GuestOS:  "ubuntu-64",
		Username: "tester",
		Password: "secret",
		Command:  "echo quick",
	})
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("GuestRun() error = %v", err)
	}
	if elapsed >= 600*time.Millisecond {
		t.Fatalf("GuestRun() took %v, want it to finish before cleanup delay", elapsed)
	}

	time.Sleep(950 * time.Millisecond)
}

func TestBackendUploadDownloadAndInventoryCapability(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "transfer-vm", "ubuntu-64", 2, 2048)
	localSource := filepath.Join(vmDir, "local.txt")
	localDest := filepath.Join(vmDir, "downloaded.txt")
	if err := os.WriteFile(localSource, []byte("upload me"), 0o600); err != nil {
		t.Fatalf("WriteFile(localSource) error = %v", err)
	}

	backend, _, _ := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT":      "Total running VMs: 1\n" + vmxPath + "\n",
		"FAKE_VMRUN_DOWNLOAD_CONTENT": "downloaded content\n",
	})

	if err := backend.Upload(context.Background(), noEmitWS, manager.UploadRequest{
		VMRef:     vmxPath,
		Username:  "tester",
		Password:  "secret",
		LocalPath: localSource,
		GuestPath: "/tmp/remote.txt",
	}); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	if err := backend.Download(context.Background(), noEmitWS, manager.DownloadRequest{
		VMRef:     vmxPath,
		Username:  "tester",
		Password:  "secret",
		GuestPath: "/tmp/remote.txt",
		LocalPath: localDest,
	}); err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	got, err := os.ReadFile(localDest)
	if err != nil {
		t.Fatalf("ReadFile(localDest) error = %v", err)
	}
	if string(got) != "downloaded content\n" {
		t.Fatalf("downloaded content = %q, want %q", string(got), "downloaded content\n")
	}

	if _, ok := any(backend).(manager.InventoryBackend); ok {
		t.Fatal("Workstation backend unexpectedly implements manager.InventoryBackend")
	}
}

func TestWorkstationIntegrationSmoke(t *testing.T) {
	if os.Getenv("XMAN_WS_INTEGRATION") != "1" {
		t.Skip("set XMAN_WS_INTEGRATION=1 to run real Workstation integration tests")
	}

	vmrunPath := os.Getenv("XMAN_WS_VMRUN")
	vmDir := os.Getenv("XMAN_WS_VM_DIR")
	if vmrunPath == "" || vmDir == "" {
		t.Skip("set XMAN_WS_VMRUN and XMAN_WS_VM_DIR for Workstation integration tests")
	}

	backend, err := NewBackend(vmrunPath, vmDir)
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}
	defer backend.Disconnect(context.Background())

	vms, err := backend.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs() error = %v", err)
	}
	if len(vms) == 0 {
		t.Fatal("ListVMs() returned no VMs")
	}

	vm, err := backend.GetVM(context.Background(), vms[0].Ref)
	if err != nil {
		t.Fatalf("GetVM() error = %v", err)
	}
	if vm.Ref == "" || vm.Name == "" {
		t.Fatalf("GetVM() returned incomplete VM info: %+v", vm)
	}
}

func newFakeVmrunBackend(t *testing.T, vmDir string, env map[string]string) (*Backend, map[string]string, string) {
	t.Helper()

	logPath := filepath.Join(t.TempDir(), "fake-vmrun.log")
	scriptPath := filepath.Join(t.TempDir(), "fake-vmrun.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail

log_file="${FAKE_VMRUN_LOG:?}"
printf '%s\n' "$*" >> "$log_file"

args=("$@")
if [[ ${#args[@]} -ge 2 && "${args[0]}" == "-T" && "${args[1]}" == "ws" ]]; then
  args=("${args[@]:2}")
fi
if [[ ${#args[@]} -ge 4 && "${args[0]}" == "-gu" ]]; then
  args=("${args[@]:4}")
fi

cmd="${args[0]:-}"
case "$cmd" in
  list)
    printf '%s' "${FAKE_VMRUN_LIST_OUTPUT:-Total running VMs: 0\n}"
    ;;
  checkToolsState)
    printf '%s\n' "${FAKE_VMRUN_TOOLS_STATE:-running}"
    ;;
  getGuestIPAddress)
    printf '%s\n' "${FAKE_VMRUN_GUEST_IP:-}"
    ;;
  readVariable)
    printf '%s\n' "${FAKE_VMRUN_READ_VARIABLE:-}"
    ;;
  listSnapshots)
    printf '%s' "${FAKE_VMRUN_SNAPSHOTS_OUTPUT:-Total snapshots: 0\n}"
    ;;
  copyFileFromGuestToHost)
    if [[ -n "${FAKE_VMRUN_COPY_FROM_GUEST_ERROR:-}" ]]; then
      printf '%s\n' "${FAKE_VMRUN_COPY_FROM_GUEST_ERROR}" >&2
      exit 1
    fi
    guest_path="${args[2]}"
    local_path="${args[3]}"
    if [[ "$guest_path" == *"exec_out_"* ]]; then
      printf '%s' "${FAKE_VMRUN_EXEC_OUTPUT:-}" > "$local_path"
    else
      printf '%s' "${FAKE_VMRUN_DOWNLOAD_CONTENT:-downloaded\n}" > "$local_path"
    fi
    ;;
  runProgramInGuest)
    if [[ -n "${FAKE_VMRUN_RUN_ERROR:-}" ]]; then
      printf '%s\n' "${FAKE_VMRUN_RUN_ERROR}" >&2
      exit 1
    fi
    if [[ -n "${FAKE_VMRUN_CLEANUP_SLEEP_SECS:-}" && "${args[2]:-}" == "/bin/sh" && "${args[3]:-}" == "-c" && "${args[4]:-}" == rm\ -f* ]]; then
      sleep "${FAKE_VMRUN_CLEANUP_SLEEP_SECS}"
    fi
    if [[ "${FAKE_VMRUN_NONZERO_EXIT:-0}" == "1" ]]; then
      printf '%s\n' "${FAKE_VMRUN_NONZERO_EXIT_TEXT:-non-zero exit code}" >&2
      exit 1
    fi
    ;;
  copyFileFromHostToGuest)
    if [[ -n "${FAKE_VMRUN_COPY_TO_GUEST_ERROR:-}" ]]; then
      printf '%s\n' "${FAKE_VMRUN_COPY_TO_GUEST_ERROR}" >&2
      exit 1
    fi
    ;;
  deleteFileInGuest|start|stop|reset|suspend|snapshot|revertToSnapshot|deleteSnapshot|installTools)
    ;;
  *)
    printf 'unexpected fake vmrun command: %s\n' "$cmd" >&2
    exit 2
    ;;
esac
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(fake-vmrun) error = %v", err)
	}

	baseEnv := map[string]string{
		"FAKE_VMRUN_LOG": logPath,
	}
	for k, v := range env {
		baseEnv[k] = v
	}
	for k, v := range baseEnv {
		t.Setenv(k, v)
	}

	backend, err := NewBackend(scriptPath, vmDir)
	if err != nil {
		t.Fatalf("NewBackend(fake vmrun) error = %v", err)
	}
	t.Cleanup(func() {
		_ = backend.Disconnect(context.Background())
	})
	return backend, baseEnv, logPath
}

func writeTestVMX(t *testing.T, rootDir, name, guestOS string, cpu, memoryMB int) string {
	t.Helper()

	vmSubdir := filepath.Join(rootDir, name)
	if err := os.MkdirAll(vmSubdir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", vmSubdir, err)
	}
	vmxPath := filepath.Join(vmSubdir, name+".vmx")
	vmx := fmt.Sprintf("displayName = %q\nguestOS = %q\nnumvcpus = %q\nmemsize = %q\n", name, guestOS, fmt.Sprint(cpu), fmt.Sprint(memoryMB))
	if err := os.WriteFile(vmxPath, []byte(vmx), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", vmxPath, err)
	}
	return vmxPath
}

func noEmitWS(int, string) {}

func countLogLines(logText, want string) int {
	count := 0
	for _, line := range strings.Split(logText, "\n") {
		if strings.TrimSpace(line) == want {
			count++
		}
	}
	return count
}

func TestBackendGuestRunToolsNotReadyReturnsHelpfulError(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "kali-vm", "debian-64", 2, 4096)

	backend, _, _ := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT": "Total running VMs: 1\n" + vmxPath + "\n",
		"FAKE_VMRUN_RUN_ERROR":   "Error: The VMware Tools are not running in the virtual machine.",
	})

	err := backend.GuestRun(context.Background(), noEmitWS, manager.RunRequest{
		VMRef:     vmxPath,
		Username:  "root",
		Password:  "secret",
		Command:   "echo ready",
		GuestOS:   "debian-64",
	})
	if err == nil {
		t.Fatal("GuestRun() error = nil, want helpful Guest Ops readiness error")
	}
	if !strings.Contains(err.Error(), "Guest Ops is not ready in the guest yet") {
		t.Fatalf("GuestRun() error = %q, want Guest Ops readiness hint", err)
	}
}

func TestBackendUploadToolsNotReadyReturnsHelpfulError(t *testing.T) {
	vmDir := t.TempDir()
	vmxPath := writeTestVMX(t, vmDir, "kali-vm", "debian-64", 2, 4096)
	localPath := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(localPath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", localPath, err)
	}

	backend, _, _ := newFakeVmrunBackend(t, vmDir, map[string]string{
		"FAKE_VMRUN_LIST_OUTPUT":         "Total running VMs: 1\n" + vmxPath + "\n",
		"FAKE_VMRUN_COPY_TO_GUEST_ERROR": "Error: The VMware Tools are not running in the virtual machine.",
	})

	err := backend.Upload(context.Background(), noEmitWS, manager.UploadRequest{
		VMRef:     vmxPath,
		Username:  "root",
		Password:  "secret",
		LocalPath: localPath,
		GuestPath: "/tmp/test.txt",
	})
	if err == nil {
		t.Fatal("Upload() error = nil, want helpful Guest Ops readiness error")
	}
	if !strings.Contains(err.Error(), "Guest Ops is not ready in the guest yet") {
		t.Fatalf("Upload() error = %q, want Guest Ops readiness hint", err)
	}
}
