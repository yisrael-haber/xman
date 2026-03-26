package workstation

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// inventoryPath returns the OS-appropriate path to VMware's inventory.vmls file.
// This is the same file Workstation uses for its own VM library.
func inventoryPath() (string, error) {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA not set")
		}
		return filepath.Join(appData, "VMware", "inventory.vmls"), nil
	default: // linux, darwin
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".vmware", "inventory.vmls"), nil
	}
}

// parseInventory reads inventory.vmls and returns the .vmx paths of all registered VMs.
// The format is a simple key=value file:
//
//	inventory.count = "3"
//	item0.path = "/home/user/vmware/Ubuntu/Ubuntu.vmx"
//	item0.type = "1"
func parseInventory(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no inventory yet — not an error
		}
		return nil, fmt.Errorf("opening inventory: %w", err)
	}
	defer f.Close()

	kvs := parseKeyValue(f)

	countStr := kvs["inventory.count"]
	if countStr == "" {
		return nil, nil
	}
	count, err := strconv.Atoi(countStr)
	if err != nil || count == 0 {
		return nil, nil
	}

	paths := make([]string, 0, count)
	for i := 0; i < count; i++ {
		// Linux/macOS use "item0.path"; Windows uses "item0.config"
		p := kvs[fmt.Sprintf("item%d.config", i)]
		if p == "" {
			p = kvs[fmt.Sprintf("item%d.path", i)]
		}
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// vmxInfo holds the fields we care about from a .vmx file.
type vmxInfo struct {
	DisplayName string
	GuestOS     string
	NumCPU      int32
	MemoryMB    int32
}

// parseVMX reads a .vmx file and extracts display name and hardware info.
func parseVMX(path string) (vmxInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return vmxInfo{}, err
	}
	defer f.Close()

	kvs := parseKeyValue(f)

	info := vmxInfo{
		DisplayName: kvs["displayName"],
		GuestOS:     kvs["guestOS"],
	}

	if n, err := strconv.Atoi(kvs["numvcpus"]); err == nil {
		info.NumCPU = int32(n)
	} else {
		info.NumCPU = 1 // default when field is absent
	}

	if m, err := strconv.Atoi(kvs["memsize"]); err == nil {
		info.MemoryMB = int32(m)
	}

	return info, nil
}

// parseKeyValue reads a VMware-style key = "value" stream into a map.
// Lines that don't match the pattern are silently skipped.
func parseKeyValue(r io.Reader) map[string]string {
	kvs := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		// strip surrounding quotes
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		kvs[key] = val
	}
	return kvs
}

// defaultVMDirs returns the standard locations where VMware stores VMs per OS.
func defaultVMDirs() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		return []string{filepath.Join(home, "Documents", "Virtual Machines")}
	case "darwin":
		return []string{
			filepath.Join(home, "Documents", "Virtual Machines"),
			filepath.Join(home, "Virtual Machines"),
		}
	default: // linux
		return []string{filepath.Join(home, "vmware")}
	}
}

// scanVMDirectories finds all .vmx files one level deep in the default VM dirs.
// Used as a fallback when inventory.vmls is absent or empty.
func scanVMDirectories() ([]string, error) {
	var paths []string
	for _, dir := range defaultVMDirs() {
		found, err := scanDirectory(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		paths = append(paths, found...)
	}
	return paths, nil
}

// scanDirectory finds all .vmx files one level deep inside a single directory.
func scanDirectory(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", dir, err)
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sub := filepath.Join(dir, entry.Name())
		files, err := os.ReadDir(sub)
		if err != nil {
			continue
		}
		for _, f := range files {
			if strings.HasSuffix(strings.ToLower(f.Name()), ".vmx") {
				paths = append(paths, filepath.Join(sub, f.Name()))
			}
		}
	}
	return paths, nil
}

// detectVmrun searches common install locations and PATH for the vmrun binary.
func detectVmrun() string {
	candidates := []string{"vmrun"} // PATH first

	switch runtime.GOOS {
	case "windows":
		candidates = append(candidates,
			`C:\Program Files (x86)\VMware\VMware Workstation\vmrun.exe`,
			`C:\Program Files\VMware\VMware Workstation\vmrun.exe`,
		)
	case "darwin":
		candidates = append(candidates,
			"/Applications/VMware Fusion.app/Contents/Library/vmrun",
		)
	default: // linux
		candidates = append(candidates,
			"/usr/bin/vmrun",
			"/usr/local/bin/vmrun",
		)
	}

	for _, c := range candidates {
		if c == "vmrun" {
			// rely on PATH — if exec.LookPath succeeds it'll be found at runtime
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "vmrun" // fall back to PATH
}
