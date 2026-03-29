package workstation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestBackendUploadDownloadAndUnsupportedInventory(t *testing.T) {
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

	if _, err := backend.ListHosts(context.Background()); err == nil {
		t.Fatal("ListHosts() error = nil, want unsupported error")
	}
	if _, err := backend.ListDatastores(context.Background()); err == nil {
		t.Fatal("ListDatastores() error = nil, want unsupported error")
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
    guest_path="${args[2]}"
    local_path="${args[3]}"
    if [[ "$guest_path" == *"exec_out_"* ]]; then
      printf '%s' "${FAKE_VMRUN_EXEC_OUTPUT:-}" > "$local_path"
    else
      printf '%s' "${FAKE_VMRUN_DOWNLOAD_CONTENT:-downloaded\n}" > "$local_path"
    fi
    ;;
  runProgramInGuest)
    if [[ "${FAKE_VMRUN_NONZERO_EXIT:-0}" == "1" ]]; then
      printf '%s\n' "${FAKE_VMRUN_NONZERO_EXIT_TEXT:-non-zero exit code}" >&2
      exit 1
    fi
    ;;
  copyFileFromHostToGuest|deleteFileInGuest|start|stop|reset|suspend|snapshot|revertToSnapshot|deleteSnapshot|installTools)
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
