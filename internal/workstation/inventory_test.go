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

func TestParseInventoryVMsSupportsVmlistHierarchy(t *testing.T) {
	tmpDir := t.TempDir()
	inventoryPath := filepath.Join(tmpDir, "inventory.vmls")
	content := `
vmlist1.config = ""
vmlist1.DisplayName = "Lab"
vmlist1.ParentID = "0"
vmlist1.ItemID = "1"
vmlist2.config = ""
vmlist2.DisplayName = "Windows"
vmlist2.ParentID = "1"
vmlist2.ItemID = "2"
vmlist3.config = "C:\VMs\win\win.vmx"
vmlist3.DisplayName = "win"
vmlist3.ParentID = "2"
vmlist3.ItemID = "3"
vmlist3.SeqID = "1"
vmlist4.config = "C:\VMs\linux\linux.vmx"
vmlist4.DisplayName = "linux"
vmlist4.ParentID = "1"
vmlist4.ItemID = "4"
vmlist4.SeqID = "0"
vmlist5.config = ""
`
	if err := os.WriteFile(inventoryPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := parseInventoryVMs(inventoryPath)
	if err != nil {
		t.Fatalf("parseInventoryVMs() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("parseInventoryVMs() len = %d, want %d (%v)", len(got), 2, got)
	}

	if got[0].Path != `C:\VMs\linux\linux.vmx` {
		t.Fatalf("first inventory path = %q, want %q", got[0].Path, `C:\VMs\linux\linux.vmx`)
	}
	if got[0].DisplayName != "linux" {
		t.Fatalf("first displayName = %q, want %q", got[0].DisplayName, "linux")
	}
	wantFirstSegments := []string{"Lab"}
	if len(got[0].PathSegments) != len(wantFirstSegments) {
		t.Fatalf("first pathSegments len = %d, want %d (%v)", len(got[0].PathSegments), len(wantFirstSegments), got[0].PathSegments)
	}
	for i := range wantFirstSegments {
		if got[0].PathSegments[i] != wantFirstSegments[i] {
			t.Fatalf("first pathSegments[%d] = %q, want %q", i, got[0].PathSegments[i], wantFirstSegments[i])
		}
	}

	wantSecondSegments := []string{"Lab", "Windows"}
	if len(got[1].PathSegments) != len(wantSecondSegments) {
		t.Fatalf("second pathSegments len = %d, want %d (%v)", len(got[1].PathSegments), len(wantSecondSegments), got[1].PathSegments)
	}
	for i := range wantSecondSegments {
		if got[1].PathSegments[i] != wantSecondSegments[i] {
			t.Fatalf("second pathSegments[%d] = %q, want %q", i, got[1].PathSegments[i], wantSecondSegments[i])
		}
	}

	paths, err := parseInventory(inventoryPath)
	if err != nil {
		t.Fatalf("parseInventory() error = %v", err)
	}
	wantPaths := []string{`C:\VMs\linux\linux.vmx`, `C:\VMs\win\win.vmx`}
	if len(paths) != len(wantPaths) {
		t.Fatalf("parseInventory() len = %d, want %d (%v)", len(paths), len(wantPaths), paths)
	}
	for i := range wantPaths {
		if paths[i] != wantPaths[i] {
			t.Fatalf("parseInventory()[%d] = %q, want %q", i, paths[i], wantPaths[i])
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

func TestHierarchyForVMXUsesConfiguredRoot(t *testing.T) {
	vmxPath := filepath.Join(string(filepath.Separator), "srv", "vms", "lab", "ubuntu", "ubuntu.vmx")

	segments, displayPath := hierarchyForVMX(vmxPath, filepath.Join(string(filepath.Separator), "srv", "vms"))

	want := []string{"lab", "ubuntu"}
	if len(segments) != len(want) {
		t.Fatalf("hierarchyForVMX() len = %d, want %d (%v)", len(segments), len(want), segments)
	}
	for i := range want {
		if segments[i] != want[i] {
			t.Fatalf("hierarchyForVMX()[%d] = %q, want %q", i, segments[i], want[i])
		}
	}
	if displayPath != "lab / ubuntu" {
		t.Fatalf("displayPath = %q, want %q", displayPath, "lab / ubuntu")
	}
}

func TestHierarchyForVMXFallsBackToHomeRelativePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	vmxPath := filepath.Join(home, "custom-vms", "team-a", "builder", "builder.vmx")
	segments, displayPath := hierarchyForVMX(vmxPath, "")

	want := []string{"custom-vms", "team-a", "builder"}
	if len(segments) != len(want) {
		t.Fatalf("hierarchyForVMX() len = %d, want %d (%v)", len(segments), len(want), segments)
	}
	for i := range want {
		if segments[i] != want[i] {
			t.Fatalf("hierarchyForVMX()[%d] = %q, want %q", i, segments[i], want[i])
		}
	}
	if displayPath != "custom-vms / team-a / builder" {
		t.Fatalf("displayPath = %q, want %q", displayPath, "custom-vms / team-a / builder")
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
