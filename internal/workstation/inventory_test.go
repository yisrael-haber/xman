package workstation

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestParseKeyValue(t *testing.T) {
	input := strings.NewReader(`
# comment
displayName = "Ubuntu Dev"
guestOS="ubuntu-64"
numvcpus = "4"
invalid line
memsize = "8192"
`)

	got := parseKeyValue(input)

	if got["displayName"] != "Ubuntu Dev" {
		t.Fatalf("displayName = %q, want %q", got["displayName"], "Ubuntu Dev")
	}
	if got["guestOS"] != "ubuntu-64" {
		t.Fatalf("guestOS = %q, want %q", got["guestOS"], "ubuntu-64")
	}
	if got["numvcpus"] != "4" {
		t.Fatalf("numvcpus = %q, want %q", got["numvcpus"], "4")
	}
	if got["memsize"] != "8192" {
		t.Fatalf("memsize = %q, want %q", got["memsize"], "8192")
	}
}

func TestParseInventorySupportsPathAndConfig(t *testing.T) {
	tmpDir := t.TempDir()
	inventoryPath := filepath.Join(tmpDir, "inventory.vmls")
	content := `
inventory.count = "3"
item0.path = "/tmp/linux-a/linux-a.vmx"
item1.config = "C:\VMs\win\win.vmx"
item2.path = "/tmp/linux-b/linux-b.vmx"
`
	if err := os.WriteFile(inventoryPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := parseInventory(inventoryPath)
	if err != nil {
		t.Fatalf("parseInventory() error = %v", err)
	}

	want := []string{
		"/tmp/linux-a/linux-a.vmx",
		`C:\VMs\win\win.vmx`,
		"/tmp/linux-b/linux-b.vmx",
	}
	if len(got) != len(want) {
		t.Fatalf("parseInventory() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseInventory()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseVMXParsesFieldsAndDefaultsCPU(t *testing.T) {
	tmpDir := t.TempDir()
	vmxPath := filepath.Join(tmpDir, "sample.vmx")
	content := `
displayName = "lab-vm"
guestOS = "ubuntu-64"
memsize = "4096"
`
	if err := os.WriteFile(vmxPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := parseVMX(vmxPath)
	if err != nil {
		t.Fatalf("parseVMX() error = %v", err)
	}
	if got.DisplayName != "lab-vm" {
		t.Fatalf("DisplayName = %q, want %q", got.DisplayName, "lab-vm")
	}
	if got.GuestOS != "ubuntu-64" {
		t.Fatalf("GuestOS = %q, want %q", got.GuestOS, "ubuntu-64")
	}
	if got.NumCPU != 1 {
		t.Fatalf("NumCPU = %d, want %d default", got.NumCPU, 1)
	}
	if got.MemoryMB != 4096 {
		t.Fatalf("MemoryMB = %d, want %d", got.MemoryMB, 4096)
	}
}

func TestScanDirectoryFindsVMXOneLevelDeep(t *testing.T) {
	tmpDir := t.TempDir()
	firstVMDir := filepath.Join(tmpDir, "vm-a")
	secondVMDir := filepath.Join(tmpDir, "vm-b")
	nestedDir := filepath.Join(firstVMDir, "nested")

	for _, dir := range []string{firstVMDir, secondVMDir, nestedDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}

	topLevelVMX := filepath.Join(firstVMDir, "vm-a.vmx")
	otherTopLevelVMX := filepath.Join(secondVMDir, "vm-b.VMX")
	deepVMX := filepath.Join(nestedDir, "too-deep.vmx")
	for _, path := range []string{topLevelVMX, otherTopLevelVMX, deepVMX} {
		if err := os.WriteFile(path, []byte("displayName = \"x\"\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
	}

	got, err := scanDirectory(tmpDir)
	if err != nil {
		t.Fatalf("scanDirectory() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("scanDirectory() len = %d, want %d (%v)", len(got), 2, got)
	}
	assertContainsPath(t, got, topLevelVMX)
	assertContainsPath(t, got, otherTopLevelVMX)
	if containsPath(got, deepVMX) {
		t.Fatalf("scanDirectory() unexpectedly included deeply nested VMX %q", deepVMX)
	}
}

func TestVmxNetVMnetsParsesAdapters(t *testing.T) {
	tmpDir := t.TempDir()
	vmxPath := filepath.Join(tmpDir, "nettest.vmx")
	content := `
ethernet0.present = "TRUE"
ethernet0.connectionType = "nat"
ethernet1.present = "true"
ethernet1.connectionType = "hostonly"
ethernet2.present = "true"
ethernet2.vnet = "vmnet12"
ethernet3.present = "false"
ethernet3.connectionType = "bridged"
`
	if err := os.WriteFile(vmxPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got := vmxNetVMnets(vmxPath)
	want := []int{8, 1, 12}
	sort.Ints(got)
	sort.Ints(want)
	if len(got) != len(want) {
		t.Fatalf("vmxNetVMnets() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("vmxNetVMnets()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestNormalizeToolsStatus(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{raw: "", want: "toolsNotRunning"},
		{raw: "running", want: "toolsOk"},
		{raw: "Installed", want: "toolsOk"},
		{raw: "out of date", want: "toolsOld"},
		{raw: "not installed", want: "toolsNotInstalled"},
		{raw: "not running", want: "toolsNotRunning"},
	}

	for _, tc := range cases {
		if got := normalizeToolsStatus(tc.raw); got != tc.want {
			t.Fatalf("normalizeToolsStatus(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestNormalizeGuestIP(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{raw: "192.168.50.10", want: "192.168.50.10"},
		{raw: `"192.168.50.11"`, want: "192.168.50.11"},
		{raw: "0.0.0.0", want: ""},
		{raw: "::", want: ""},
		{raw: "not-an-ip", want: ""},
	}

	for _, tc := range cases {
		if got := normalizeGuestIP(tc.raw); got != tc.want {
			t.Fatalf("normalizeGuestIP(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestSplitSnapRef(t *testing.T) {
	vmRef, snapName := splitSnapRef("/tmp/vms/example.vmx|snap-a")
	if vmRef != "/tmp/vms/example.vmx" || snapName != "snap-a" {
		t.Fatalf("splitSnapRef() = (%q, %q), want (%q, %q)", vmRef, snapName, "/tmp/vms/example.vmx", "snap-a")
	}

	vmRef, snapName = splitSnapRef("snap-a")
	if vmRef != "snap-a" || snapName != "snap-a" {
		t.Fatalf("splitSnapRef(no separator) = (%q, %q), want duplicate ref", vmRef, snapName)
	}
}

func TestParseVMnetNumberAndType(t *testing.T) {
	if n, ok := parseVMnetNumber("vmnet8"); !ok || n != 8 {
		t.Fatalf("parseVMnetNumber(vmnet8) = (%d, %v), want (8, true)", n, ok)
	}
	if n, ok := parseVMnetNumber("VMware Network Adapter VMnet1"); !ok || n != 1 {
		t.Fatalf("parseVMnetNumber(Windows adapter) = (%d, %v), want (1, true)", n, ok)
	}
	if _, ok := parseVMnetNumber("eth0"); ok {
		t.Fatal("parseVMnetNumber(eth0) unexpectedly succeeded")
	}

	if got := vmnetType(0); got != "bridged" {
		t.Fatalf("vmnetType(0) = %q, want %q", got, "bridged")
	}
	if got := vmnetType(1); got != "host-only" {
		t.Fatalf("vmnetType(1) = %q, want %q", got, "host-only")
	}
	if got := vmnetType(8); got != "nat" {
		t.Fatalf("vmnetType(8) = %q, want %q", got, "nat")
	}
	if got := vmnetType(12); got != "custom" {
		t.Fatalf("vmnetType(12) = %q, want %q", got, "custom")
	}
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

func assertContainsPath(t *testing.T, paths []string, want string) {
	t.Helper()
	if !containsPath(paths, want) {
		t.Fatalf("paths %v do not contain %q", paths, want)
	}
}
