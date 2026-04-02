package workstation

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"xman/internal/executil"
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

	cacheMu sync.RWMutex
	cache   map[string]guestRuntimeInfo

	traceLogger  *log.Logger
	traceCloser  io.Closer
	traceSession string
	traceSeq     atomic.Uint64
}

type guestRuntimeInfo struct {
	ToolsStatus string
	IPAddress   string
	RefreshedAt time.Time
}

var (
	_ manager.Backend             = (*Backend)(nil)
	_ manager.GuestOpsBackend     = (*Backend)(nil)
	_ manager.ToolsInstallBackend = (*Backend)(nil)
)

const (
	guestRuntimeCacheTTL     = 15 * time.Second
	guestRuntimeQueryTimeout = 2 * time.Second
	vmListDetailConcurrency  = 4
	vmListRefreshPerCall     = 1
)

// NewBackend validates that vmrun is available and returns a ready Backend.
// If vmrunPath is empty, common install locations and PATH are tried.
// vmDir optionally overrides where VMs are searched; leave empty for defaults.
func NewBackend(vmrunPath, vmDir string) (*Backend, error) {
	if vmrunPath == "" {
		vmrunPath = detectVmrun()
	}

	traceLogger, traceCloser := newTraceLogger(hasPerfLogArg())

	// Verify the binary works
	cmd := exec.Command(vmrunPath, "list")
	configureCmd(cmd)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("vmrun not available at %q: %w", vmrunPath, err)
	}

	backend := &Backend{
		vmrunPath:    vmrunPath,
		vmDir:        vmDir,
		cache:        make(map[string]guestRuntimeInfo),
		traceLogger:  traceLogger,
		traceCloser:  traceCloser,
		traceSession: newTraceSessionID(),
	}
	backend.traceEvent(
		"backend_ready",
		"vmrun_path", vmrunPath,
		"vm_dir", vmDir,
		"cache_ttl", guestRuntimeCacheTTL,
		"query_timeout", guestRuntimeQueryTimeout,
		"detail_concurrency", vmListDetailConcurrency,
		"refresh_per_call", vmListRefreshPerCall,
	)
	return backend, nil
}

func (b *Backend) DisplayName() string { return "Local Workstation" }

func (b *Backend) BackendType() string { return "workstation" }

func (b *Backend) Capabilities() manager.Capabilities {
	return manager.Capabilities{GuestOps: true, Inventory: false, ToolsInstall: true, Console: false}
}

func (b *Backend) Disconnect(_ context.Context) error {
	b.traceEvent("backend_disconnect_begin")
	if b.traceCloser != nil {
		if syncer, ok := b.traceCloser.(interface{ Sync() error }); ok {
			_ = syncer.Sync()
		}
		return b.traceCloser.Close()
	}
	return nil
} // stateless

func traceLogFilename() string {
	return fmt.Sprintf("xman_log_%s.txt", time.Now().Format("20060102_150405"))
}

func newTraceSessionID() string {
	return fmt.Sprintf("pid-%d-%d", os.Getpid(), time.Now().UnixNano())
}

func hasPerfLogArg() bool {
	for _, arg := range os.Args[1:] {
		normalized := strings.TrimSpace(strings.ToLower(arg))
		if normalized == "perflog" || normalized == "--perflog" {
			return true
		}
	}
	return false
}

func traceLogCandidates() []string {
	filename := traceLogFilename()
	candidates := make([]string, 0, 4)

	if exePath, err := os.Executable(); err == nil && exePath != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), filename))
	}

	if wd, err := os.Getwd(); err == nil && wd != "" {
		candidates = append(candidates, filepath.Join(wd, filename))
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, "Desktop", filename))
		candidates = append(candidates, filepath.Join(home, filename))
	}

	candidates = append(candidates, filepath.Join(os.TempDir(), filename))
	return candidates
}

func newTraceLogger(enabled bool) (*log.Logger, io.Closer) {
	if !enabled {
		return nil, nil
	}
	for _, path := range traceLogCandidates() {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			log.Printf("[workstation trace] failed to open trace log %q: %v", path, err)
			continue
		}
		logger := log.New(f, "", log.LstdFlags)
		logger.Printf("[workstation trace] writing timings to %s", path)
		return logger, f
	}
	return log.New(os.Stderr, "", log.LstdFlags), nil
}

func (b *Backend) traceEnabled() bool {
	return b.traceLogger != nil
}

func (b *Backend) nextTraceID() uint64 {
	return b.traceSeq.Add(1)
}

func (b *Backend) traceEvent(event string, fields ...any) {
	if !b.traceEnabled() {
		return
	}
	prefix := []any{"session", b.traceSession, "event", event}
	b.traceLogger.Printf("[workstation trace] %s", formatTraceFields(append(prefix, fields...)...))
}

func (b *Backend) traceTiming(op string, started time.Time, fields ...any) {
	if !b.traceEnabled() {
		return
	}
	prefix := []any{"session", b.traceSession, "op", op, "elapsed", time.Since(started).Round(time.Millisecond)}
	b.traceLogger.Printf("[workstation timing] %s", formatTraceFields(append(prefix, fields...)...))
}

func formatTraceFields(fields ...any) string {
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, 0, (len(fields)+1)/2)
	for i := 0; i < len(fields); i += 2 {
		key := fmt.Sprint(fields[i])
		value := "<missing>"
		if i+1 < len(fields) {
			value = traceFieldValue(fields[i+1])
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, " ")
}

func traceFieldValue(value any) string {
	switch v := value.(type) {
	case string:
		return strconv.Quote(v)
	case []string:
		if len(v) == 0 {
			return "[]"
		}
		quoted := make([]string, len(v))
		for i, item := range v {
			quoted[i] = strconv.Quote(item)
		}
		return "[" + strings.Join(quoted, ",") + "]"
	case time.Duration:
		return strconv.Quote(v.String())
	case time.Time:
		return strconv.Quote(v.Format(time.RFC3339Nano))
	case error:
		if v == nil {
			return `""`
		}
		return strconv.Quote(v.Error())
	case nil:
		return `""`
	default:
		return strconv.Quote(fmt.Sprint(v))
	}
}

func sanitizeArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	out := append([]string(nil), args...)
	for i := 0; i < len(out)-1; i++ {
		switch out[i] {
		case "-gu":
			out[i+1] = "<redacted-user>"
		case "-gp":
			out[i+1] = "<redacted-pass>"
		}
	}
	return out
}

func contextDeadlineString(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return ""
	}
	return deadline.Format(time.RFC3339Nano)
}

func vmLabel(vmx string) string {
	if vmx == "" {
		return ""
	}
	return filepath.Base(vmx)
}

func vmLabels(vmxPaths []string) []string {
	if len(vmxPaths) == 0 {
		return nil
	}
	labels := make([]string, len(vmxPaths))
	for i, vmx := range vmxPaths {
		labels[i] = vmLabel(vmx)
	}
	return labels
}

func keysFromSet(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, vmLabel(key))
	}
	sort.Strings(keys)
	return keys
}

func truncateForLog(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	return text[:limit-3] + "..."
}

// --- vmrun helpers ---

// runContext executes vmrun with the given args and returns trimmed stdout.
func (b *Backend) runContext(ctx context.Context, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	started := time.Now()
	cmdID := b.nextTraceID()
	safeArgs := sanitizeArgs(args)
	b.traceEvent("vmrun_start", "cmd_id", cmdID, "args", safeArgs, "deadline", contextDeadlineString(ctx))
	cmd := exec.CommandContext(ctx, b.vmrunPath, args...)
	configureCmd(cmd)
	out, err := cmd.Output()
	stdoutText := strings.TrimSpace(string(out))
	stdoutLines := 0
	if stdoutText != "" {
		stdoutLines = len(strings.Split(stdoutText, "\n"))
	}
	b.traceTiming("vmrun", started, "cmd_id", cmdID, "args", safeArgs, "stdout_bytes", len(out), "stdout_lines", stdoutLines, "success", err == nil)
	if err != nil {
		if ctx.Err() != nil {
			b.traceEvent("vmrun_context_done", "cmd_id", cmdID, "args", safeArgs, "ctx_error", ctx.Err())
			return "", ctx.Err()
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderrText := strings.TrimSpace(string(exitErr.Stderr))
			b.traceEvent("vmrun_exit_error", "cmd_id", cmdID, "args", safeArgs, "stderr", stderrText, "stdout", stdoutText)
			if msg := strings.TrimSpace(string(exitErr.Stderr)); msg != "" {
				return "", fmt.Errorf("%s", msg)
			}
			if msg := strings.TrimSpace(string(out)); msg != "" {
				return "", fmt.Errorf("%s", msg)
			}
		}
		b.traceEvent("vmrun_error", "cmd_id", cmdID, "args", safeArgs, "error", err)
		return "", err
	}
	b.traceEvent("vmrun_success", "cmd_id", cmdID, "args", safeArgs, "stdout_preview", truncateForLog(stdoutText, 160))
	return strings.TrimSpace(string(out)), nil
}

func normalizeToolsStatus(raw string) string {
	state := strings.ToLower(strings.TrimSpace(raw))

	switch {
	case state == "", strings.Contains(state, "not running"), strings.Contains(state, "stopped"):
		return "toolsNotRunning"
	case strings.Contains(state, "not installed"), strings.Contains(state, "notinstalled"), strings.Contains(state, "missing"):
		return "toolsNotInstalled"
	case strings.Contains(state, "old"), strings.Contains(state, "out of date"), strings.Contains(state, "outdated"):
		return "toolsOld"
	case strings.Contains(state, "running"), strings.Contains(state, "installed"):
		// vmrun can report "Installed" instead of "Running" for headless/nogui
		// Workstation/Fusion VMs even when guest ops are still available.
		return "toolsOk"
	default:
		return "toolsNotRunning"
	}
}

func guestOpsReadyFromToolsStatus(status string) bool {
	return status == "toolsOk" || status == "toolsOld"
}

func normalizeGuestIP(raw string) string {
	ip := strings.TrimSpace(strings.Trim(raw, `"`))
	if ip == "" {
		return ""
	}
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.IsUnspecified() {
		return ""
	}
	return parsed.String()
}

func (b *Backend) cachedRuntime(vmx string) (guestRuntimeInfo, bool) {
	b.cacheMu.RLock()
	defer b.cacheMu.RUnlock()
	info, ok := b.cache[vmx]
	return info, ok
}

func (b *Backend) storeRuntime(vmx string, info guestRuntimeInfo) {
	b.cacheMu.Lock()
	defer b.cacheMu.Unlock()
	b.cache[vmx] = info
	b.traceEvent("runtime_cache_store", "vm", vmLabel(vmx), "tools_status", info.ToolsStatus, "ip_address", info.IPAddress, "refreshed_at", info.RefreshedAt)
}

func (b *Backend) clearRuntime(vmx string) {
	b.cacheMu.Lock()
	defer b.cacheMu.Unlock()
	if _, ok := b.cache[vmx]; ok {
		delete(b.cache, vmx)
		b.traceEvent("runtime_cache_clear", "vm", vmLabel(vmx))
	}
}

func (b *Backend) runtimeForList(ctx context.Context, vmx string) guestRuntimeInfo {
	if cached, ok := b.cachedRuntime(vmx); ok {
		b.traceEvent("runtime_cache_hit", "vm", vmLabel(vmx), "age", time.Since(cached.RefreshedAt).Round(time.Millisecond), "tools_status", cached.ToolsStatus, "ip_address", cached.IPAddress)
		return cached
	}
	b.traceEvent("runtime_cache_miss", "vm", vmLabel(vmx))
	return guestRuntimeInfo{ToolsStatus: "toolsNotRunning"}
}

func (b *Backend) queryToolsState(ctx context.Context, vmx string) (string, bool) {
	started := time.Now()
	out, err := b.runContext(ctx, ws("checkToolsState", vmx)...)
	normalized := normalizeToolsStatus(out)
	b.traceTiming("checkToolsState", started, "vm", vmLabel(vmx), "raw", out, "normalized", normalized, "success", err == nil)
	if err != nil {
		b.traceEvent("tools_state_error", "vm", vmLabel(vmx), "error", err)
		return "", false
	}
	return normalized, true
}

func (b *Backend) loadRuntimeDetails(ctx context.Context, vmx string) (guestRuntimeInfo, bool) {
	started := time.Now()
	defer func() {
		b.traceTiming("loadRuntimeDetails", started, "vm", vmLabel(vmx))
	}()

	queryCtx, cancel := context.WithTimeout(ctx, guestRuntimeQueryTimeout)
	defer cancel()
	b.traceEvent("load_runtime_begin", "vm", vmLabel(vmx), "deadline", contextDeadlineString(queryCtx))

	toolsStatus, ok := b.queryToolsState(queryCtx, vmx)
	if !ok {
		b.traceEvent("load_runtime_skipped", "vm", vmLabel(vmx), "reason", "tools_query_failed")
		return guestRuntimeInfo{}, false
	}

	info := guestRuntimeInfo{
		ToolsStatus: toolsStatus,
		RefreshedAt: time.Now(),
	}
	if toolsStatus == "toolsOk" || toolsStatus == "toolsOld" {
		info.IPAddress = b.resolveGuestIP(queryCtx, vmx)
	}
	b.traceEvent("load_runtime_complete", "vm", vmLabel(vmx), "tools_status", info.ToolsStatus, "ip_address", info.IPAddress)
	return info, true
}

func (b *Backend) vmInfoFromPath(ctx context.Context, vmxPath string, running map[string]struct{}, refreshRuntime bool, metadata *inventoryVM) manager.VMInfo {
	info := manager.VMInfo{Ref: vmxPath, VMXPath: vmxPath}
	localVMXPath := localPathForVMX(vmxPath)
	if metadata != nil {
		info.PathSegments = append([]string(nil), metadata.PathSegments...)
		info.DisplayPath = strings.Join(info.PathSegments, " / ")
	} else {
		info.PathSegments, info.DisplayPath = hierarchyForVMX(localVMXPath, b.vmDir)
	}

	vmxData, err := parseVMX(localVMXPath)
	if err == nil {
		info.Name = vmxData.DisplayName
		info.GuestOS = vmxData.GuestOS
		info.NumCPU = vmxData.NumCPU
		info.MemoryMB = vmxData.MemoryMB
		info.Notes = vmxData.Notes
		info.Firmware = vmxData.Firmware
		info.HardwareVersion = vmxData.HardwareVersion
		info.UUID = vmxData.UUID
		info.VMXPath = vmxPath
		if len(vmxData.NetworkAdapters) > 0 {
			info.NetworkAdapters = make([]manager.VMNetworkAdapter, 0, len(vmxData.NetworkAdapters))
			for _, adapter := range vmxData.NetworkAdapters {
				info.NetworkAdapters = append(info.NetworkAdapters, manager.VMNetworkAdapter{
					ID:          adapter.ID,
					Label:       adapter.Label,
					NetworkID:   adapter.NetworkID,
					Network:     adapter.Network,
					NetworkType: adapter.NetworkType,
					MACAddress:  adapter.MACAddress,
					Connected:   adapter.Connected,
				})
			}
		}
	} else if metadata != nil && metadata.DisplayName != "" {
		info.Name = metadata.DisplayName
	}

	if _, on := running[vmxPath]; on {
		info.PowerState = "poweredOn"
		b.traceEvent("vm_state_detected", "vm", vmLabel(vmxPath), "state", info.PowerState, "refresh_runtime", refreshRuntime)

		runtimeInfo := b.runtimeForList(ctx, vmxPath)
		info.ToolsStatus = runtimeInfo.ToolsStatus
		info.IPAddress = runtimeInfo.IPAddress

		if refreshRuntime {
			if refreshed, ok := b.loadRuntimeDetails(ctx, vmxPath); ok {
				info.ToolsStatus = refreshed.ToolsStatus
				info.GuestOpsReady = guestOpsReadyFromToolsStatus(refreshed.ToolsStatus)
				info.IPAddress = refreshed.IPAddress
				b.storeRuntime(vmxPath, refreshed)
			} else {
				b.traceEvent("vm_runtime_refresh_failed", "vm", vmLabel(vmxPath))
			}
		}

		if info.ToolsStatus == "" {
			info.ToolsStatus = "toolsNotRunning"
		}
		info.GuestOpsReady = guestOpsReadyFromToolsStatus(info.ToolsStatus)
		b.traceEvent("vm_info_ready", "vm", vmLabel(vmxPath), "state", info.PowerState, "tools_status", info.ToolsStatus, "ip_address", info.IPAddress)
		return info
	}

	if isSuspended(localVMXPath) {
		info.PowerState = "suspended"
		info.ToolsStatus = "toolsNotRunning"
		b.clearRuntime(vmxPath)
		b.traceEvent("vm_state_detected", "vm", vmLabel(vmxPath), "state", info.PowerState)
		return info
	}

	info.PowerState = "poweredOff"
	info.ToolsStatus = "toolsNotRunning"
	b.clearRuntime(vmxPath)
	b.traceEvent("vm_state_detected", "vm", vmLabel(vmxPath), "state", info.PowerState)
	return info
}

func (b *Backend) readGuestVariable(ctx context.Context, vmx, scope, key string) string {
	started := time.Now()
	out, err := b.runContext(ctx, ws("readVariable", vmx, scope, key)...)
	b.traceTiming("readGuestVariable", started, "vm", vmLabel(vmx), "scope", scope, "key", key, "success", err == nil, "value", truncateForLog(strings.TrimSpace(out), 120))
	if err != nil {
		b.traceEvent("read_guest_variable_error", "vm", vmLabel(vmx), "scope", scope, "key", key, "error", err)
		return ""
	}
	return strings.TrimSpace(out)
}

func (b *Backend) resolveGuestIP(ctx context.Context, vmx string) string {
	started := time.Now()
	defer func() {
		b.traceTiming("resolveGuestIP", started, "vm", vmLabel(vmx))
	}()

	if ip, err := b.runContext(ctx, ws("getGuestIPAddress", vmx)...); err == nil {
		if normalized := normalizeGuestIP(ip); normalized != "" {
			b.traceEvent("resolve_guest_ip_success", "vm", vmLabel(vmx), "source", "getGuestIPAddress", "ip_address", normalized)
			return normalized
		}
		b.traceEvent("resolve_guest_ip_empty", "vm", vmLabel(vmx), "source", "getGuestIPAddress", "raw", truncateForLog(strings.TrimSpace(ip), 120))
	} else {
		b.traceEvent("resolve_guest_ip_error", "vm", vmLabel(vmx), "source", "getGuestIPAddress", "error", err)
	}

	// Fallbacks for Workstation/Fusion headless mode, where getGuestIPAddress
	// can fail even though VMware Tools guest variables are populated.
	candidates := []struct {
		scope string
		key   string
	}{
		{scope: "guestVar", key: "ip"},
		{scope: "runtimeConfig", key: "guestinfo.ip"},
		{scope: "runtimeConfig", key: "guestinfo.ipAddress"},
	}

	for _, candidate := range candidates {
		if normalized := normalizeGuestIP(b.readGuestVariable(ctx, vmx, candidate.scope, candidate.key)); normalized != "" {
			b.traceEvent("resolve_guest_ip_success", "vm", vmLabel(vmx), "source", candidate.scope+":"+candidate.key, "ip_address", normalized)
			return normalized
		}
	}

	b.traceEvent("resolve_guest_ip_failed", "vm", vmLabel(vmx))
	return ""
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
	return b.runningVMSetContext(context.Background())
}

func (b *Backend) runningVMSetContext(ctx context.Context) (map[string]struct{}, error) {
	out, err := b.runContext(ctx, "list")
	if err != nil {
		b.traceEvent("running_vm_set_error", "error", err)
		return nil, err
	}
	set := make(map[string]struct{})
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ".vmx") {
			set[line] = struct{}{}
		}
	}
	b.traceEvent("running_vm_set_ready", "count", len(set), "vms", keysFromSet(set))
	return set, nil
}

// --- VM lifecycle ---

func (b *Backend) ListVMs(ctx context.Context) ([]manager.VMInfo, error) {
	started := time.Now()
	inventoryStarted := time.Now()
	inventorySource := "inventory"
	b.traceEvent("list_vms_begin", "vm_dir", b.vmDir)
	var inventoryEntries []inventoryVM
	invPath, err := inventoryPath()
	if err != nil {
		b.traceEvent("list_vms_inventory_path_error", "error", err)
	} else {
		inventoryEntries, err = parseInventoryVMs(invPath)
		if err != nil {
			b.traceEvent("list_vms_inventory_parse_error", "inventory_path", invPath, "error", err)
			if b.vmDir == "" {
				return nil, err
			}
			inventoryEntries = nil
		}
	}
	inventoryMetadata := inventoryMetadataByPath(inventoryEntries)

	var vmxPaths []string
	if b.vmDir != "" {
		inventorySource = "vm_dir"
		vmxPaths, err = scanDirectory(b.vmDir)
	} else {
		for _, entry := range inventoryEntries {
			vmxPaths = append(vmxPaths, entry.Path)
		}
		if len(vmxPaths) == 0 {
			inventorySource = "scan_fallback"
			vmxPaths, err = scanVMDirectories()
		}
	}
	if err != nil {
		b.traceEvent("list_vms_inventory_error", "source", inventorySource, "error", err)
		return nil, err
	}
	inventoryDur := time.Since(inventoryStarted)
	b.traceEvent("list_vms_inventory_ready", "source", inventorySource, "inventory_path", invPath, "vm_count", len(vmxPaths), "vms", vmLabels(vmxPaths))

	runningStarted := time.Now()
	running, err := b.runningVMSetContext(ctx)
	if err != nil {
		return nil, err
	}
	runningDur := time.Since(runningStarted)

	// Fetch per-VM details concurrently to avoid O(2N) serial vmrun calls.
	type result struct {
		idx  int
		info manager.VMInfo
	}
	results := make([]manager.VMInfo, len(vmxPaths))
	ch := make(chan result, len(vmxPaths))
	sem := make(chan struct{}, vmListDetailConcurrency)

	staleRefreshBudget := vmListRefreshPerCall
	var staleMu sync.Mutex

	var wg sync.WaitGroup
	detailStarted := time.Now()
	for i, vmx := range vmxPaths {
		wg.Add(1)
		go func(idx int, vmxPath string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cached, hasCached := b.cachedRuntime(vmxPath)
			cacheAge := time.Duration(0)
			if hasCached {
				cacheAge = time.Since(cached.RefreshedAt)
			}
			needsRefresh := !hasCached || cacheAge > guestRuntimeCacheTTL
			refreshRuntime := false
			if needsRefresh {
				staleMu.Lock()
				if staleRefreshBudget > 0 {
					staleRefreshBudget--
					refreshRuntime = true
				}
				staleMu.Unlock()
			}
			b.traceEvent(
				"list_vms_vm_plan",
				"vm", vmLabel(vmxPath),
				"has_cached_runtime", hasCached,
				"cache_age", cacheAge.Round(time.Millisecond),
				"needs_refresh", needsRefresh,
				"refresh_runtime", refreshRuntime,
			)

			var vmMeta *inventoryVM
			if entry, ok := inventoryMetadataForPath(inventoryMetadata, vmxPath); ok {
				entryCopy := entry
				vmMeta = &entryCopy
			}

			info := b.vmInfoFromPath(ctx, vmxPath, running, refreshRuntime, vmMeta)

			ch <- result{idx: idx, info: info}
		}(i, vmx)
	}

	wg.Wait()
	detailDur := time.Since(detailStarted)
	close(ch)
	for r := range ch {
		results[r.idx] = r.info
	}
	b.traceTiming(
		"ListVMs",
		started,
		"inventory", inventoryDur.Round(time.Millisecond),
		"running", runningDur.Round(time.Millisecond),
		"detail", detailDur.Round(time.Millisecond),
		"vm_count", len(vmxPaths),
		"powered_on", len(running),
		"inventory_source", inventorySource,
	)
	b.traceEvent("list_vms_complete", "vm_count", len(results), "powered_on", len(running))
	return results, nil
}

func (b *Backend) GetVM(ctx context.Context, vmRef string) (manager.VMInfo, error) {
	started := time.Now()
	b.traceEvent("get_vm_begin", "vm", vmLabel(vmRef))
	running, err := b.runningVMSetContext(ctx)
	if err != nil {
		b.traceEvent("get_vm_running_set_error", "vm", vmLabel(vmRef), "error", err)
		return manager.VMInfo{}, err
	}

	var vmMeta *inventoryVM
	if invPath, err := inventoryPath(); err == nil {
		if entries, err := parseInventoryVMs(invPath); err == nil {
			if entry, ok := inventoryMetadataForPath(inventoryMetadataByPath(entries), vmRef); ok {
				entryCopy := entry
				vmMeta = &entryCopy
			}
		}
	}

	info := b.vmInfoFromPath(ctx, vmRef, running, true, vmMeta)
	if info.Name == "" {
		if _, err := os.Stat(localPathForVMX(vmRef)); err != nil {
			b.traceEvent("get_vm_not_found", "vm", vmLabel(vmRef), "error", err)
			return manager.VMInfo{}, fmt.Errorf("VM %q not found", vmRef)
		}
	}
	b.traceTiming("GetVM", started, "vm", vmLabel(vmRef), "power_state", info.PowerState, "tools_status", info.ToolsStatus, "ip_address", info.IPAddress)
	return info, nil
}

// isSuspended checks for a .vmss suspend-state file alongside the .vmx.
func isSuspended(vmx string) bool {
	dir := filepath.Dir(vmx)
	base := strings.TrimSuffix(filepath.Base(vmx), ".vmx")
	_, err := os.Stat(filepath.Join(dir, base+".vmss"))
	return err == nil
}

func (b *Backend) PowerOn(ctx context.Context, vmRef string) error {
	started := time.Now()
	b.traceEvent("power_on_begin", "vm", vmLabel(vmRef))
	// Use "nogui" so vmrun returns immediately after starting the VM rather
	// than blocking until the Workstation window is closed.
	_, err := b.runContext(ctx, ws("start", vmRef, "nogui")...)
	b.traceTiming("PowerOn", started, "vm", vmLabel(vmRef), "success", err == nil)
	if err != nil {
		b.traceEvent("power_on_error", "vm", vmLabel(vmRef), "error", err)
	}
	return err
}

func normalizeRequestedFirmware(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "bios":
		return "bios"
	case "efi", "uefi":
		return "efi"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func (b *Backend) ListVMNetworkOptions(_ context.Context, vmRef string) ([]manager.VMNetworkOption, error) {
	if _, err := os.Stat(localPathForVMX(vmRef)); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("VM %q not found", vmRef)
		}
		return nil, fmt.Errorf("checking VMX path: %w", err)
	}
	return workstationNetworkOptions()
}

func (b *Backend) UpdateVMNetwork(ctx context.Context, emit jobs.EmitFn, req manager.VMNetworkUpdateRequest) error {
	emit(10, "Loading current VM network settings...")
	info, err := b.GetVM(ctx, req.VMRef)
	if err != nil {
		return err
	}
	if info.PowerState != "poweredOff" {
		return fmt.Errorf("Workstation network changes can only be applied while powered off")
	}

	localVMXPath := localPathForVMX(req.VMRef)
	current, err := parseVMX(localVMXPath)
	if err != nil {
		return fmt.Errorf("reading VMX configuration: %w", err)
	}

	var adapter *vmxNetworkAdapter
	for i := range current.NetworkAdapters {
		if current.NetworkAdapters[i].ID == req.AdapterID {
			adapter = &current.NetworkAdapters[i]
			break
		}
	}
	if adapter == nil {
		return fmt.Errorf("network adapter %q not found", req.AdapterID)
	}

	connectionType, vnet, err := vmxNetworkSettings(req.NetworkID)
	if err != nil {
		return err
	}

	updates := make(map[string]*string)
	hasChanges := false

	if adapter.NetworkID != req.NetworkID {
		value := connectionType
		updates[req.AdapterID+".connectionType"] = &value
		updates[req.AdapterID+".vnet"] = vnet
		hasChanges = true
	}

	if adapter.Connected != req.Connected {
		value := "false"
		if req.Connected {
			value = "true"
		}
		updates[req.AdapterID+".startConnected"] = &value
		hasChanges = true
	}

	if !hasChanges {
		emit(100, "Network attachment already matches the requested values.")
		return nil
	}

	emit(55, "Writing VMX network settings...")
	if err := writeVMXUpdates(localVMXPath, updates); err != nil {
		return fmt.Errorf("writing VMX configuration: %w", err)
	}

	b.clearRuntime(req.VMRef)
	emit(100, "Network attachment updated.")
	return nil
}

func (b *Backend) UpdateVMConfig(ctx context.Context, emit jobs.EmitFn, req manager.VMConfigUpdateRequest) error {
	emit(10, "Loading current VM configuration...")
	info, err := b.GetVM(ctx, req.VMRef)
	if err != nil {
		return err
	}
	if info.PowerState != "poweredOff" {
		return fmt.Errorf("Workstation VM configuration can only be edited while powered off")
	}

	localVMXPath := localPathForVMX(req.VMRef)
	current, err := parseVMX(localVMXPath)
	if err != nil {
		return fmt.Errorf("reading VMX configuration: %w", err)
	}

	updates := make(map[string]*string)
	hasChanges := false

	nextName := strings.TrimSpace(req.Name)
	if nextName != "" && nextName != current.DisplayName {
		value := nextName
		updates["displayName"] = &value
		hasChanges = true
	}

	nextNotes := strings.TrimSpace(req.Notes)
	if nextNotes != current.Notes {
		if nextNotes == "" {
			updates["annotation"] = nil
		} else {
			value := encodeVMXAnnotation(nextNotes)
			updates["annotation"] = &value
		}
		hasChanges = true
	}

	if req.NumCPU != current.NumCPU {
		value := strconv.Itoa(int(req.NumCPU))
		updates["numvcpus"] = &value
		hasChanges = true
	}

	if req.MemoryMB != current.MemoryMB {
		value := strconv.Itoa(int(req.MemoryMB))
		updates["memsize"] = &value
		hasChanges = true
	}

	currentFirmware := normalizeRequestedFirmware(current.Firmware)
	requestedFirmware := normalizeRequestedFirmware(req.Firmware)
	if requestedFirmware != "" && requestedFirmware != currentFirmware {
		value := requestedFirmware
		updates["firmware"] = &value
		hasChanges = true
	}

	if !hasChanges {
		emit(100, "Configuration already matches the requested values.")
		return nil
	}

	emit(55, "Writing VMX configuration...")
	if err := writeVMXUpdates(localVMXPath, updates); err != nil {
		return fmt.Errorf("writing VMX configuration: %w", err)
	}

	b.clearRuntime(req.VMRef)
	emit(100, "Configuration updated.")
	return nil
}

func (b *Backend) PowerOff(ctx context.Context, vmRef string) error {
	started := time.Now()
	b.traceEvent("power_off_begin", "vm", vmLabel(vmRef))
	_, err := b.runContext(ctx, ws("stop", vmRef, "hard")...)
	b.traceTiming("PowerOff", started, "vm", vmLabel(vmRef), "success", err == nil)
	if err != nil {
		b.traceEvent("power_off_error", "vm", vmLabel(vmRef), "error", err)
	}
	return err
}

func (b *Backend) Reset(ctx context.Context, vmRef string) error {
	started := time.Now()
	b.traceEvent("reset_begin", "vm", vmLabel(vmRef))
	_, err := b.runContext(ctx, ws("reset", vmRef, "hard")...)
	b.traceTiming("Reset", started, "vm", vmLabel(vmRef), "success", err == nil)
	if err != nil {
		b.traceEvent("reset_error", "vm", vmLabel(vmRef), "error", err)
	}
	return err
}

func (b *Backend) Suspend(ctx context.Context, vmRef string) error {
	started := time.Now()
	b.traceEvent("suspend_begin", "vm", vmLabel(vmRef))
	_, err := b.runContext(ctx, ws("suspend", vmRef)...)
	b.traceTiming("Suspend", started, "vm", vmLabel(vmRef), "success", err == nil)
	if err != nil {
		b.traceEvent("suspend_error", "vm", vmLabel(vmRef), "error", err)
	}
	return err
}

// --- Snapshots ---
// vmrun identifies snapshots by name. Ref == Name for this backend.
// Tree depth and IsCurrent are not available from vmrun's output.

func (b *Backend) ListSnapshots(ctx context.Context, vmRef string) ([]manager.SnapshotInfo, error) {
	started := time.Now()
	b.traceEvent("list_snapshots_begin", "vm", vmLabel(vmRef))
	out, err := b.runContext(ctx, ws("listSnapshots", vmRef)...)
	if err != nil {
		b.traceEvent("list_snapshots_error", "vm", vmLabel(vmRef), "error", err)
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
	b.traceTiming("ListSnapshots", started, "vm", vmLabel(vmRef), "snapshot_count", len(snaps))
	return snaps, nil
}

func (b *Backend) CreateSnapshot(ctx context.Context, emit jobs.EmitFn, req manager.CreateSnapshotRequest) error {
	started := time.Now()
	b.traceEvent("create_snapshot_begin", "vm", vmLabel(req.VMRef), "snapshot_name", req.Name)
	emit(10, "Creating snapshot...")
	_, err := b.runContext(ctx, ws("snapshot", req.VMRef, req.Name)...)
	if err != nil {
		b.traceEvent("create_snapshot_error", "vm", vmLabel(req.VMRef), "snapshot_name", req.Name, "error", err)
		return err
	}
	emit(100, fmt.Sprintf("Snapshot %q created", req.Name))
	b.traceTiming("CreateSnapshot", started, "vm", vmLabel(req.VMRef), "snapshot_name", req.Name)
	return nil
}

func (b *Backend) RevertSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string) error {
	started := time.Now()
	vmRef, snapName := splitSnapRef(snapRef)
	b.traceEvent("revert_snapshot_begin", "vm", vmLabel(vmRef), "snapshot_name", snapName)
	emit(10, "Reverting to snapshot...")
	_, err := b.runContext(ctx, ws("revertToSnapshot", vmRef, snapName)...)
	if err != nil {
		b.traceEvent("revert_snapshot_error", "vm", vmLabel(vmRef), "snapshot_name", snapName, "error", err)
		return err
	}
	emit(100, "Reverted successfully")
	b.traceTiming("RevertSnapshot", started, "vm", vmLabel(vmRef), "snapshot_name", snapName)
	return nil
}

func (b *Backend) DeleteSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string, _ bool) error {
	started := time.Now()
	vmRef, snapName := splitSnapRef(snapRef)
	b.traceEvent("delete_snapshot_begin", "vm", vmLabel(vmRef), "snapshot_name", snapName)
	emit(10, "Deleting snapshot...")
	_, err := b.runContext(ctx, ws("deleteSnapshot", vmRef, snapName)...)
	if err != nil {
		b.traceEvent("delete_snapshot_error", "vm", vmLabel(vmRef), "snapshot_name", snapName, "error", err)
		return err
	}
	emit(100, "Deleted successfully")
	b.traceTiming("DeleteSnapshot", started, "vm", vmLabel(vmRef), "snapshot_name", snapName)
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

// guestRunEnv returns whether the guest is Windows and a temp output path,
// based on the guest OS string from the VMX or request.
// Handles both VMX short IDs ("win10-64") and display names ("Microsoft Windows 10 (64-bit)").
func guestRunEnv(guestOS string) (isWin bool, outPath, pidPath string) {
	outName := fmt.Sprintf("exec_out_%d.txt", time.Now().UnixNano())
	pidName := fmt.Sprintf("exec_pid_%d.txt", time.Now().UnixNano())
	if manager.IsWindows(guestOS) {
		return true, `C:\Users\Public\` + outName, `C:\Users\Public\` + pidName
	}
	return false, "/tmp/" + outName, "/tmp/" + pidName
}

func (b *Backend) cancelGuestRun(vmRef, username, password string, isWin bool, pidPath string) {
	if strings.TrimSpace(pidPath) == "" {
		return
	}

	killCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if isWin {
		args := append(
			guest(username, password, "runProgramInGuest", vmRef, manager.WinPSExePath),
			manager.WinPSStopPIDFromFileArgList(pidPath)...,
		)
		_, _ = b.runContext(killCtx, args...)
	} else {
		killCmd := fmt.Sprintf("test -f %s && kill $(cat %s) >/dev/null 2>&1 || true", manager.ShQuote(pidPath), manager.ShQuote(pidPath))
		_, _ = b.runContext(killCtx, guest(username, password,
			"runProgramInGuest", vmRef,
			"/bin/sh", "-c", killCmd)...)
	}

	_, _ = b.runContext(killCtx, guest(username, password,
		"deleteFileInGuest", vmRef, pidPath)...)
}

func (b *Backend) Upload(ctx context.Context, emit jobs.EmitFn, req manager.UploadRequest) error {
	started := time.Now()
	b.traceEvent("upload_begin", "vm", vmLabel(req.VMRef), "local_path", req.LocalPath, "guest_path", req.GuestPath)
	emit(10, "Copying file to guest...")
	_, err := b.runContext(ctx, guest(req.Username, req.Password,
		"copyFileFromHostToGuest", req.VMRef, req.LocalPath, req.GuestPath)...)
	if err != nil {
		b.traceEvent("upload_error", "vm", vmLabel(req.VMRef), "local_path", req.LocalPath, "guest_path", req.GuestPath, "error", err)
		return fmt.Errorf("upload: %w", err)
	}
	emit(100, "Upload complete.")
	b.traceTiming("Upload", started, "vm", vmLabel(req.VMRef), "local_path", req.LocalPath, "guest_path", req.GuestPath)
	return nil
}

func (b *Backend) Download(ctx context.Context, emit jobs.EmitFn, req manager.DownloadRequest) error {
	started := time.Now()
	b.traceEvent("download_begin", "vm", vmLabel(req.VMRef), "guest_path", req.GuestPath, "local_path", req.LocalPath)
	emit(10, "Copying file from guest...")
	_, err := b.runContext(ctx, guest(req.Username, req.Password,
		"copyFileFromGuestToHost", req.VMRef, req.GuestPath, req.LocalPath)...)
	if err != nil {
		b.traceEvent("download_error", "vm", vmLabel(req.VMRef), "guest_path", req.GuestPath, "local_path", req.LocalPath, "error", err)
		return fmt.Errorf("download: %w", err)
	}
	emit(100, "Download complete.")
	b.traceTiming("Download", started, "vm", vmLabel(req.VMRef), "guest_path", req.GuestPath, "local_path", req.LocalPath)
	return nil
}

func (b *Backend) GuestRun(ctx context.Context, emit jobs.EmitFn, req manager.RunRequest) error {
	started := time.Now()
	isWin, outPath, pidPath := guestRunEnv(req.GuestOS)
	b.traceEvent("guest_run_begin", "vm", vmLabel(req.VMRef), "guest_os", req.GuestOS, "guest_is_windows", isWin, "output_path", outPath, "pid_path", pidPath, "command", truncateForLog(req.Command, 200))

	emit(10, "Executing command...")
	var runErr error
	if isWin {
		// cmd.exe I/O redirection doesn't work in VMware's headless guest-exec
		// session (no console attached). Use PowerShell with Out-File instead.
		// Pass flags as separate args — vmrun re-quotes a single string, which
		// makes PowerShell treat it as a positional parameter instead of flags.
		args := append(
			guest(req.Username, req.Password, "runProgramInGuest", req.VMRef, manager.WinPSExePath),
			manager.WinPSCmdArgListWithPID(req.Command, outPath, pidPath)...,
		)
		_, runErr = b.runContext(ctx, args...)
	} else {
		runCmd := fmt.Sprintf("printf '%%s' $$ > %s; exec /bin/sh %s", manager.ShQuote(pidPath), manager.PosixCaptureArgs(req.Command, outPath))
		_, runErr = b.runContext(ctx, guest(req.Username, req.Password,
			"runProgramInGuest", req.VMRef,
			"/bin/sh", "-c", runCmd)...)
	}

	if ctx.Err() != nil {
		b.traceEvent("guest_run_cancel_requested", "vm", vmLabel(req.VMRef), "pid_path", pidPath, "ctx_error", ctx.Err())
		b.cancelGuestRun(req.VMRef, req.Username, req.Password, isWin, pidPath)
		return ctx.Err()
	}

	// vmrun exits with code 1 whenever the guest program itself exits non-zero,
	// but the output file may still exist. Only bail out for genuine vmrun errors
	// (auth failures, tools not running, etc.) — not guest exit-code errors.
	if runErr != nil && !strings.Contains(runErr.Error(), "non-zero exit code") {
		b.traceEvent("guest_run_vmrun_error", "vm", vmLabel(req.VMRef), "error", runErr)
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

	_, downloadErr := b.runContext(ctx, guest(req.Username, req.Password,
		"copyFileFromGuestToHost", req.VMRef, outPath, tmpPath)...)
	if downloadErr != nil {
		b.traceEvent("guest_run_download_error", "vm", vmLabel(req.VMRef), "error", downloadErr)
		if runErr != nil {
			return fmt.Errorf("running command: %w\ndownload: %w", runErr, downloadErr)
		}
		return fmt.Errorf("downloading output: %w", downloadErr)
	}

	// best-effort cleanup of the temp file in the guest
	_, _ = b.runContext(ctx, guest(req.Username, req.Password,
		"deleteFileInGuest", req.VMRef, outPath)...)
	_, _ = b.runContext(ctx, guest(req.Username, req.Password,
		"deleteFileInGuest", req.VMRef, pidPath)...)

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("reading output: %w", err)
	}

	output := executil.NormalizeCapturedOutput(data)
	if runErr != nil {
		b.traceEvent("guest_run_nonzero_exit", "vm", vmLabel(req.VMRef), "output_preview", truncateForLog(output, 200), "error", runErr)
		emit(95, output+"\n\n["+runErr.Error()+"]")
		emit(100, "Command finished with non-zero exit status.")
		b.traceTiming("GuestRun", started, "vm", vmLabel(req.VMRef), "success", true, "nonzero_exit", true, "output_bytes", len(data))
		return nil
	}
	b.traceEvent("guest_run_complete", "vm", vmLabel(req.VMRef), "output_preview", truncateForLog(output, 200))
	emit(95, output)
	emit(100, "Command completed.")
	b.traceTiming("GuestRun", started, "vm", vmLabel(req.VMRef), "success", true, "nonzero_exit", false, "output_bytes", len(data))
	return nil
}

func (b *Backend) ListNetworks(_ context.Context) (manager.NetworkSummary, error) {
	started := time.Now()
	b.traceEvent("list_networks_begin")
	hostVMnets, err := discoverHostVMnets()
	if err != nil {
		b.traceEvent("list_networks_interface_error", "error", err)
		return manager.NetworkSummary{}, err
	}

	// Step 2: Map VMnet number → VM names by parsing every known VMX file.
	vmnetVMs := make(map[int][]string)
	if vmxPaths, err := b.allVMXPaths(); err == nil {
		for _, vmxPath := range vmxPaths {
			info, err := parseVMX(vmxPath)
			if err != nil {
				continue
			}
			name := info.DisplayName
			if name == "" {
				name = strings.TrimSuffix(filepath.Base(vmxPath), ".vmx")
			}
			for _, n := range vmxNetVMnets(vmxPath) {
				vmnetVMs[n] = manager.AppendUnique(vmnetVMs[n], name)
			}
		}
	} else {
		b.traceEvent("list_networks_all_vmx_error", "error", err)
	}

	// Step 3: Assemble sorted switch list.
	switches := make([]manager.SwitchInfo, 0, len(hostVMnets.numbers))
	for _, n := range hostVMnets.numbers {
		e := hostVMnets.details[n]
		switches = append(switches, manager.SwitchInfo{
			Name:    fmt.Sprintf("VMnet%d", n),
			Type:    vmnetType(n),
			MTU:     e.mtu,
			Uplinks: e.addrs,
			Hosts:   vmnetVMs[n],
		})
	}

	b.traceTiming("ListNetworks", started, "switch_count", len(switches))
	return manager.NetworkSummary{Switches: switches}, nil
}

func workstationNetworkOptions() ([]manager.VMNetworkOption, error) {
	options := []manager.VMNetworkOption{
		{ID: "bridged", Name: "Bridged (VMnet0)", Type: "Bridged"},
		{ID: "nat", Name: "NAT (VMnet8)", Type: "NAT"},
		{ID: "hostonly", Name: "Host-only (VMnet1)", Type: "Host-only"},
	}

	hostVMnets, err := discoverHostVMnets()
	if err != nil {
		return nil, err
	}

	for _, n := range hostVMnets.numbers {
		if n == 0 || n == 1 || n == 8 {
			continue
		}
		vmnetName := fmt.Sprintf("VMnet%d", n)
		options = append(options, manager.VMNetworkOption{
			ID:   "custom:" + strings.ToLower(vmnetName),
			Name: "Custom (" + vmnetName + ")",
			Type: "Custom",
		})
	}

	return options, nil
}

type vmnetInterfaceInfo struct {
	mtu   int32
	addrs []string
}

type hostVMnetInventory struct {
	numbers []int
	details map[int]vmnetInterfaceInfo
}

func discoverHostVMnets() (hostVMnetInventory, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return hostVMnetInventory{}, fmt.Errorf("listing network interfaces: %w", err)
	}

	details := make(map[int]vmnetInterfaceInfo)
	for _, iface := range ifaces {
		n, ok := parseVMnetNumber(iface.Name)
		if !ok {
			continue
		}

		info := vmnetInterfaceInfo{mtu: int32(iface.MTU)}
		if addrs, err := iface.Addrs(); err == nil {
			for _, addr := range addrs {
				info.addrs = append(info.addrs, addr.String())
			}
		}
		details[n] = info
	}

	numbers := make([]int, 0, len(details))
	for n := range details {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)

	return hostVMnetInventory{
		numbers: numbers,
		details: details,
	}, nil
}

// allVMXPaths returns the VMX paths from the configured directory or the default inventory.
func (b *Backend) allVMXPaths() ([]string, error) {
	if b.vmDir != "" {
		b.traceEvent("all_vmx_paths_begin", "source", "vm_dir", "vm_dir", b.vmDir)
		return scanDirectory(b.vmDir)
	}
	invPath, err := inventoryPath()
	if err != nil {
		b.traceEvent("all_vmx_paths_inventory_path_error", "error", err)
		return nil, err
	}
	paths, err := parseInventory(invPath)
	if err != nil || len(paths) == 0 {
		b.traceEvent("all_vmx_paths_scan_fallback", "inventory_path", invPath, "parse_error", err, "inventory_count", len(paths))
		return scanVMDirectories()
	}
	b.traceEvent("all_vmx_paths_ready", "source", "inventory", "inventory_path", invPath, "vm_count", len(paths), "vms", vmLabels(paths))
	return paths, nil
}

// vmxNetVMnets returns the VMnet numbers used by all present network adapters in a VMX file.
func vmxNetVMnets(vmxPath string) []int {
	data, err := os.ReadFile(vmxPath)
	if err != nil {
		return nil
	}

	type adapter struct {
		present        bool
		connectionType string
		vnet           string
	}
	adapters := make(map[string]*adapter)

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		eq := strings.Index(line, " = ")
		if eq < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:eq]))
		val := strings.Trim(strings.TrimSpace(line[eq+3:]), `"`)

		if !strings.HasPrefix(key, "ethernet") {
			continue
		}
		dot := strings.IndexByte(key, '.')
		if dot < 0 {
			continue
		}
		id, field := key[:dot], key[dot+1:]
		if adapters[id] == nil {
			adapters[id] = &adapter{}
		}
		switch field {
		case "present":
			adapters[id].present = strings.EqualFold(val, "true")
		case "connectiontype":
			adapters[id].connectionType = strings.ToLower(val)
		case "vnet":
			adapters[id].vnet = strings.ToLower(val)
		}
	}

	var out []int
	for _, a := range adapters {
		if !a.present {
			continue
		}
		// Explicit vnet= takes priority over connectionType mapping.
		if a.vnet != "" && strings.HasPrefix(a.vnet, "vmnet") {
			if n, err := strconv.Atoi(a.vnet[5:]); err == nil {
				out = append(out, n)
				continue
			}
		}
		switch a.connectionType {
		case "nat":
			out = append(out, 8)
		case "hostonly":
			out = append(out, 1)
		case "bridged":
			out = append(out, 0)
		}
	}
	return out
}

// parseVMnetNumber extracts the VMnet index from an interface name.
// Handles Linux ("vmnet1") and Windows ("VMware Network Adapter VMnet1").
func parseVMnetNumber(name string) (int, bool) {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "vmnet") {
		if n, err := strconv.Atoi(name[5:]); err == nil {
			return n, true
		}
	}
	if idx := strings.Index(lower, "vmnet"); idx >= 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(name[idx+5:])); err == nil {
			return n, true
		}
	}
	return 0, false
}

func vmnetType(n int) string {
	switch n {
	case 0:
		return "bridged"
	case 1:
		return "host-only"
	case 8:
		return "nat"
	default:
		return "custom"
	}
}

func (b *Backend) InstallTools(ctx context.Context, emit jobs.EmitFn, vmRef string) error {
	started := time.Now()
	b.traceEvent("install_tools_begin", "vm", vmLabel(vmRef))
	info, err := parseVMX(vmRef)
	if err == nil && !strings.HasPrefix(strings.ToLower(info.GuestOS), "win") {
		b.traceEvent("install_tools_rejected", "vm", vmLabel(vmRef), "guest_os", info.GuestOS, "reason", "non_windows_guest")
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
	b.traceTiming("InstallToolsMount", started, "vm", vmLabel(vmRef), "success", cmdErr == nil)

	// Both a quick non-zero exit (no guest agent to respond) and a timeout (agent
	// never responded) mean the same thing: tools are not installed. In either case
	// the ISO is already mounted at the hypervisor level; guide the user.
	if cmdErr != nil {
		b.traceEvent("install_tools_mount_incomplete", "vm", vmLabel(vmRef), "error", cmdErr)
		emit(100, "VMware Tools ISO mounted. Open the CD-ROM drive inside the guest and run setup64.exe (or setup.exe on 32-bit) to complete installation.")
		return nil
	}

	emit(100, "VMware Tools installation initiated in guest.")
	b.traceEvent("install_tools_complete", "vm", vmLabel(vmRef))
	return nil
}
