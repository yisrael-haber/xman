package workstation

import (
	"bufio"
	"fmt"
	"io"
	"os"
	slashpath "path"
	"path/filepath"
	"runtime"
	"sort"
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
		candidates, err := inventoryPathCandidates()
		if err != nil {
			return "", err
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
		if len(candidates) == 0 {
			return "", fmt.Errorf("inventory.vmls not found")
		}
		return candidates[0], nil
	}
}

func inventoryPathCandidates() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	candidates := []string{filepath.Join(home, ".vmware", "inventory.vmls")}
	windowsCandidates, err := filepath.Glob("/mnt/c/Users/*/AppData/Roaming/VMware/inventory.vmls")
	if err == nil {
		sort.Strings(windowsCandidates)
		candidates = append(candidates, windowsCandidates...)
	}
	return candidates, nil
}

type inventoryVM struct {
	Path         string
	DisplayName  string
	PathSegments []string
	SeqID        int
}

type vmlistRecord struct {
	Key         string
	Config      string
	DisplayName string
	ParentID    string
	ItemID      string
	SeqID       int
	HasSeqID    bool
}

// parseInventory reads inventory.vmls and returns the .vmx paths of all registered VMs.
// The format is a simple key=value file:
//
//	inventory.count = "3"
//	item0.path = "/home/user/vmware/Ubuntu/Ubuntu.vmx"
//	item0.type = "1"
func parseInventory(path string) ([]string, error) {
	entries, err := parseInventoryVMs(path)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	return paths, nil
}

func parseInventoryVMs(path string) ([]inventoryVM, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no inventory yet — not an error
		}
		return nil, fmt.Errorf("opening inventory: %w", err)
	}
	defer f.Close()

	kvs := parseKeyValue(f)
	if entries := parseVmlistInventory(kvs); len(entries) > 0 {
		return entries, nil
	}
	return parseLegacyInventory(kvs), nil
}

func parseLegacyInventory(kvs map[string]string) []inventoryVM {
	countStr := kvs["inventory.count"]
	if countStr == "" {
		return nil
	}
	count, err := strconv.Atoi(countStr)
	if err != nil || count == 0 {
		return nil
	}

	entries := make([]inventoryVM, 0, count)
	for i := 0; i < count; i++ {
		// Linux/macOS use "item0.path"; Windows uses "item0.config"
		p := kvs[fmt.Sprintf("item%d.config", i)]
		if p == "" {
			p = kvs[fmt.Sprintf("item%d.path", i)]
		}
		if p != "" {
			entries = append(entries, inventoryVM{Path: p, SeqID: i})
		}
	}
	return entries
}

func parseVmlistInventory(kvs map[string]string) []inventoryVM {
	records := make(map[string]*vmlistRecord)
	for key, value := range kvs {
		prefix, field, ok := strings.Cut(key, ".")
		if !ok || !strings.HasPrefix(prefix, "vmlist") {
			continue
		}

		record := records[prefix]
		if record == nil {
			record = &vmlistRecord{Key: prefix}
			records[prefix] = record
		}

		switch field {
		case "config":
			record.Config = value
		case "DisplayName":
			record.DisplayName = value
		case "ParentID":
			record.ParentID = value
		case "ItemID":
			record.ItemID = value
		case "SeqID":
			if seqID, err := strconv.Atoi(value); err == nil {
				record.SeqID = seqID
				record.HasSeqID = true
			}
		}
	}

	if len(records) == 0 {
		return nil
	}

	byItemID := make(map[string]*vmlistRecord, len(records))
	recordList := make([]*vmlistRecord, 0, len(records))
	for _, record := range records {
		if record.ItemID != "" {
			byItemID[record.ItemID] = record
		}
		recordList = append(recordList, record)
	}

	sort.Slice(recordList, func(i, j int) bool {
		left, right := recordList[i], recordList[j]
		switch {
		case left.HasSeqID && right.HasSeqID && left.SeqID != right.SeqID:
			return left.SeqID < right.SeqID
		case left.HasSeqID != right.HasSeqID:
			return left.HasSeqID
		default:
			return left.Key < right.Key
		}
	})

	entries := make([]inventoryVM, 0, len(recordList))
	for _, record := range recordList {
		if record.Config == "" {
			continue
		}
		entries = append(entries, inventoryVM{
			Path:         record.Config,
			DisplayName:  record.DisplayName,
			PathSegments: vmlistPathSegments(record.ParentID, byItemID),
			SeqID:        record.SeqID,
		})
	}
	return entries
}

func vmlistPathSegments(parentID string, byItemID map[string]*vmlistRecord) []string {
	if parentID == "" || parentID == "0" {
		return nil
	}

	var segments []string
	visited := make(map[string]struct{})
	currentID := parentID
	for currentID != "" && currentID != "0" {
		if _, seen := visited[currentID]; seen {
			break
		}
		visited[currentID] = struct{}{}

		record := byItemID[currentID]
		if record == nil {
			break
		}
		if record.Config == "" && record.DisplayName != "" {
			segments = append([]string{record.DisplayName}, segments...)
		}
		currentID = record.ParentID
	}
	return segments
}

// vmxInfo holds the fields we care about from a .vmx file.
type vmxInfo struct {
	DisplayName     string
	GuestOS         string
	NumCPU          int32
	MemoryMB        int32
	Notes           string
	Firmware        string
	HardwareVersion string
	UUID            string
	NetworkAdapters []vmxNetworkAdapter
}

type vmxNetworkAdapter struct {
	ID          string
	Label       string
	NetworkID   string
	Network     string
	NetworkType string
	MACAddress  string
	Connected   bool
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
		DisplayName:     kvs["displayName"],
		GuestOS:         kvs["guestOS"],
		Notes:           cleanVMXAnnotation(kvs["annotation"]),
		Firmware:        formatVMXFirmware(kvs["firmware"]),
		HardwareVersion: formatVMXHardwareVersion(kvs["virtualHW.version"]),
		UUID:            firstNonEmpty(kvs["uuid.bios"], kvs["uuid.location"]),
		NetworkAdapters: parseVMXNetworkAdapters(kvs),
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

func parseVMXNetworkAdapters(kvs map[string]string) []vmxNetworkAdapter {
	type rawAdapter struct {
		present           bool
		hasPresent        bool
		connectionType    string
		vnet              string
		macAddress        string
		generatedAddress  string
		startConnected    bool
		hasStartConnected bool
	}

	adapters := make(map[string]*rawAdapter)
	for key, value := range kvs {
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		if !strings.HasPrefix(lowerKey, "ethernet") {
			continue
		}
		dot := strings.IndexByte(lowerKey, '.')
		if dot < 0 {
			continue
		}

		id, field := lowerKey[:dot], lowerKey[dot+1:]
		adapter := adapters[id]
		if adapter == nil {
			adapter = &rawAdapter{}
			adapters[id] = adapter
		}

		switch field {
		case "present":
			adapter.present = strings.EqualFold(value, "true")
			adapter.hasPresent = true
		case "connectiontype":
			adapter.connectionType = strings.ToLower(strings.TrimSpace(value))
		case "vnet":
			adapter.vnet = strings.ToLower(strings.TrimSpace(value))
		case "address":
			adapter.macAddress = value
		case "generatedaddress":
			adapter.generatedAddress = value
		case "startconnected":
			adapter.startConnected = !strings.EqualFold(value, "false") && value != "0"
			adapter.hasStartConnected = true
		}
	}

	if len(adapters) == 0 {
		return nil
	}

	ids := make([]string, 0, len(adapters))
	for id := range adapters {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ethernetAdapterIndex(ids[i]) < ethernetAdapterIndex(ids[j])
	})

	out := make([]vmxNetworkAdapter, 0, len(ids))
	for _, id := range ids {
		adapter := adapters[id]
		if adapter == nil || (adapter.hasPresent && !adapter.present) {
			continue
		}

		networkName, networkType := formatVMXNetwork(adapter.connectionType, adapter.vnet)
		macAddress := adapter.macAddress
		if macAddress == "" {
			macAddress = adapter.generatedAddress
		}

		labelIndex := ethernetAdapterIndex(id) + 1
		if labelIndex <= 0 {
			labelIndex = len(out) + 1
		}

		connected := true
		if adapter.hasStartConnected {
			connected = adapter.startConnected
		}

		networkID, networkName, networkType := vmxNetworkSelection(adapter.connectionType, adapter.vnet)

		out = append(out, vmxNetworkAdapter{
			ID:          id,
			Label:       fmt.Sprintf("Network adapter %d", labelIndex),
			NetworkID:   networkID,
			Network:     networkName,
			NetworkType: networkType,
			MACAddress:  macAddress,
			Connected:   connected,
		})
	}

	return out
}

func ethernetAdapterIndex(id string) int {
	raw := strings.TrimPrefix(strings.ToLower(id), "ethernet")
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 1 << 30
	}
	return n
}

func formatVMXNetwork(connectionType, vnet string) (string, string) {
	_, name, kind := vmxNetworkSelection(connectionType, vnet)
	return name, kind
}

func vmxNetworkSelection(connectionType, vnet string) (string, string, string) {
	vnet = strings.ToLower(strings.TrimSpace(vnet))
	connectionType = strings.ToLower(strings.TrimSpace(connectionType))

	switch {
	case vnet != "" && strings.HasPrefix(vnet, "vmnet"):
		switch vnet {
		case "vmnet0":
			return "bridged", "Bridged (VMnet0)", "Bridged"
		case "vmnet1":
			return "hostonly", "Host-only (VMnet1)", "Host-only"
		case "vmnet8":
			return "nat", "NAT (VMnet8)", "NAT"
		default:
			return "custom:" + vnet, "Custom (" + strings.ToUpper(vnet[:2]) + vnet[2:] + ")", "Custom"
		}
	case connectionType == "bridged":
		return "bridged", "Bridged (VMnet0)", "Bridged"
	case connectionType == "hostonly":
		return "hostonly", "Host-only (VMnet1)", "Host-only"
	case connectionType == "nat":
		return "nat", "NAT (VMnet8)", "NAT"
	case connectionType == "custom":
		return "", "Custom network", "Custom"
	default:
		return "", "", ""
	}
}

func vmxNetworkSettings(networkID string) (string, *string, error) {
	normalized := strings.ToLower(strings.TrimSpace(networkID))
	switch normalized {
	case "bridged":
		return "bridged", nil, nil
	case "hostonly":
		return "hostonly", nil, nil
	case "nat":
		return "nat", nil, nil
	}
	if strings.HasPrefix(normalized, "custom:vmnet") {
		vnet := strings.TrimPrefix(normalized, "custom:")
		return "custom", &vnet, nil
	}
	return "", nil, fmt.Errorf("unsupported Workstation network %q", networkID)
}

func formatVMXFirmware(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "efi", "uefi":
		return "UEFI"
	case "bios":
		return "BIOS"
	default:
		return raw
	}
}

func formatVMXHardwareVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if _, err := strconv.Atoi(raw); err == nil {
		return "v" + raw
	}
	return raw
}

func cleanVMXAnnotation(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		"|0A", "\n",
		"|0D", "",
		"&#10;", "\n",
		"\\n", "\n",
	)
	return strings.TrimSpace(replacer.Replace(raw))
}

func encodeVMXAnnotation(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"\r\n", "\n",
		"\r", "\n",
		"\n", "|0A",
	)
	return replacer.Replace(raw)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func writeVMXUpdates(path string, updates map[string]*string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	applied := make(map[string]bool, len(updates))

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		eq := strings.Index(trimmed, "=")
		if eq < 0 {
			continue
		}

		key := strings.TrimSpace(trimmed[:eq])
		nextValue, ok := updates[key]
		if !ok {
			continue
		}

		applied[key] = true
		if nextValue == nil {
			lines[i] = ""
			continue
		}

		lines[i] = fmt.Sprintf(`%s = %s`, key, vmxQuotedValue(*nextValue))
	}

	for key, nextValue := range updates {
		if applied[key] || nextValue == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf(`%s = %s`, key, vmxQuotedValue(*nextValue)))
	}

	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func vmxQuotedValue(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
	return `"` + escaped + `"`
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

func hierarchyForVMX(vmxPath, configuredRoot string) ([]string, string) {
	vmDir := filepath.Clean(filepath.Dir(localPathForVMX(vmxPath)))
	root := bestHierarchyRoot(vmDir, configuredRoot)
	if root == "" {
		return nil, ""
	}

	rel, err := filepath.Rel(root, vmDir)
	if err != nil || rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
		return nil, ""
	}

	segments := splitHierarchySegments(rel)
	if len(segments) == 0 {
		return nil, ""
	}
	return segments, strings.Join(segments, " / ")
}

func bestHierarchyRoot(vmDir, configuredRoot string) string {
	candidates := make([]string, 0, len(defaultVMDirs())+2)
	if configuredRoot != "" {
		candidates = append(candidates, configuredRoot)
	}
	candidates = append(candidates, defaultVMDirs()...)
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, home)
	}

	best := ""
	for _, candidate := range candidates {
		if !pathWithinRoot(vmDir, candidate) {
			continue
		}
		if len(filepath.Clean(candidate)) > len(best) {
			best = filepath.Clean(candidate)
		}
	}
	if best != "" {
		return best
	}

	volume := filepath.VolumeName(vmDir)
	if volume != "" {
		return volume + string(filepath.Separator)
	}
	return string(filepath.Separator)
}

func pathWithinRoot(path, root string) bool {
	if path == "" || root == "" {
		return false
	}

	cleanPath := normalizeComparablePath(path)
	cleanRoot := normalizeComparablePath(root)
	if cleanPath == cleanRoot {
		return true
	}
	return strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator))
}

func normalizeComparablePath(value string) string {
	cleaned := filepath.Clean(value)
	if runtime.GOOS == "windows" {
		return strings.ToLower(cleaned)
	}
	return cleaned
}

func localPathForVMX(vmxPath string) string {
	if runtime.GOOS == "windows" || len(vmxPath) < 3 || vmxPath[1] != ':' {
		return vmxPath
	}

	drive := strings.ToLower(string(vmxPath[0]))
	rest := strings.ReplaceAll(vmxPath[2:], `\`, `/`)
	rest = strings.TrimPrefix(rest, "/")
	return slashpath.Clean("/mnt/" + drive + "/" + rest)
}

func inventoryLookupKey(value string) string {
	if value == "" {
		return ""
	}
	key := strings.ReplaceAll(strings.TrimSpace(value), `\`, `/`)
	key = slashpath.Clean(key)
	return strings.ToLower(key)
}

func inventoryLookupKeys(value string) []string {
	keys := []string{inventoryLookupKey(value)}
	local := localPathForVMX(value)
	localKey := inventoryLookupKey(local)
	if localKey != "" && localKey != keys[0] {
		keys = append(keys, localKey)
	}
	return keys
}

func inventoryMetadataByPath(entries []inventoryVM) map[string]inventoryVM {
	if len(entries) == 0 {
		return nil
	}

	metadata := make(map[string]inventoryVM, len(entries)*2)
	for _, entry := range entries {
		for _, key := range inventoryLookupKeys(entry.Path) {
			if key != "" {
				metadata[key] = entry
			}
		}
	}
	return metadata
}

func inventoryMetadataForPath(metadata map[string]inventoryVM, path string) (inventoryVM, bool) {
	if len(metadata) == 0 {
		return inventoryVM{}, false
	}
	for _, key := range inventoryLookupKeys(path) {
		if entry, ok := metadata[key]; ok {
			return entry, true
		}
	}
	return inventoryVM{}, false
}

func splitHierarchySegments(value string) []string {
	if value == "" || value == "." {
		return nil
	}

	raw := strings.Split(filepath.Clean(value), string(filepath.Separator))
	segments := make([]string, 0, len(raw))
	for _, segment := range raw {
		if segment == "" || segment == "." {
			continue
		}
		segments = append(segments, segment)
	}
	return segments
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
