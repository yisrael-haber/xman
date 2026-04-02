package vcenter

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xman/internal/executil"
	"xman/internal/jobs"
	"xman/internal/manager"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/fault"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/guest/toolbox"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	gsoap "github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	consoleHostSourceConnectedHost = "connected_host"
	consoleHostSourceReportedFQDN  = "reported_fqdn"
	consoleHostSourceSessionHost   = "session_host"
	inventoryPathCacheTTL          = 30 * time.Second
	guestOpsReadyPollInterval      = 3 * time.Second
	guestOpsWarmupWait             = 10 * time.Second
	guestOpsBootstrapWait          = 45 * time.Second
	guestOpsUpgradeWait            = 90 * time.Second
)

// Backend implements manager.Backend against a vCenter session.
type Backend struct {
	session     *Session
	pathCacheMu sync.RWMutex
	pathCache   map[string]cachedPathSegments
}

type cachedPathSegments struct {
	segments  []string
	expiresAt time.Time
}

var (
	_ manager.Backend             = (*Backend)(nil)
	_ manager.GuestOpsBackend     = (*Backend)(nil)
	_ manager.InventoryBackend    = (*Backend)(nil)
	_ manager.ToolsInstallBackend = (*Backend)(nil)
	_ manager.ConsoleBackend      = (*Backend)(nil)
)

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
	return &Backend{
		session:   s,
		pathCache: make(map[string]cachedPathSegments),
	}, nil
}

func (b *Backend) DisplayName() string {
	return "vCenter @ " + b.session.Host()
}

func (b *Backend) BackendType() string { return "vcenter" }

func (b *Backend) Capabilities() manager.Capabilities {
	return manager.Capabilities{GuestOps: true, Inventory: true, ToolsInstall: true, Console: true}
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
		"name",
		"config.name", "config.guestFullName",
		"config.hardware.numCPU", "config.hardware.memoryMB",
		"runtime.powerState", "guest.toolsStatus", "guest.guestOperationsReady", "guest.ipAddress",
		"parent", "parentVApp",
	}, &vms)
	if err != nil {
		return nil, fmt.Errorf("listing VMs: %w", err)
	}

	pathSegmentsByParent, err := b.resolveVMParentPathSegments(ctx, client, vms)
	if err != nil {
		return nil, err
	}

	out := make([]manager.VMInfo, 0, len(vms))
	for _, obj := range vms {
		out = append(out, toVMInfo(obj, pathSegmentsByParent[pathKey(vmParentRef(obj))], nil))
	}
	return out, nil
}

func toVMInfo(obj mo.VirtualMachine, pathSegments []string, distributedPortgroupsByKey map[string]vCenterDistributedPortgroupRef) manager.VMInfo {
	info := manager.VMInfo{
		Ref:          obj.Reference().Value,
		PathSegments: pathSegments,
		DisplayPath:  strings.Join(pathSegments, " / "),
		PowerState:   string(obj.Runtime.PowerState),
	}
	if name := strings.TrimSpace(obj.Name); name != "" {
		info.Name = name
	}
	if obj.Config != nil {
		if info.Name == "" {
			info.Name = obj.Config.Name
		}
		info.GuestOS = obj.Config.GuestFullName
		info.NumCPU = obj.Config.Hardware.NumCPU
		info.MemoryMB = obj.Config.Hardware.MemoryMB
		info.Firmware = formatVCenterFirmware(obj.Config.Firmware)
		info.HardwareVersion = obj.Config.Version
		info.UUID = obj.Config.Uuid
		info.Notes = strings.TrimSpace(obj.Config.Annotation)
	}
	if obj.Guest != nil {
		info.ToolsStatus = string(obj.Guest.ToolsStatus)
		if obj.Guest.GuestOperationsReady != nil {
			info.GuestOpsReady = *obj.Guest.GuestOperationsReady
		}
		info.IPAddress = obj.Guest.IpAddress
		info.GuestHostname = obj.Guest.HostName
		if info.HardwareVersion == "" {
			info.HardwareVersion = obj.Guest.HwVersion
		}
	}
	info.NetworkAdapters = buildVCenterNetworkAdapters(obj, distributedPortgroupsByKey)
	return info
}

func normalizeVCenterFirmware(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "bios":
		return "bios"
	case "efi", "uefi":
		return "efi"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func formatVCenterFirmware(raw string) string {
	switch normalizeVCenterFirmware(raw) {
	case "efi":
		return "UEFI"
	case "bios":
		return "BIOS"
	default:
		return raw
	}
}

type vCenterGuestNICDetails struct {
	network     string
	ipAddresses []string
}

func buildVCenterNetworkAdapters(obj mo.VirtualMachine, distributedPortgroupsByKey map[string]vCenterDistributedPortgroupRef) []manager.VMNetworkAdapter {
	if obj.Config == nil || len(obj.Config.Hardware.Device) == 0 {
		return nil
	}

	guestNICs := vCenterGuestNICDetailsByMAC(obj)
	devices := object.VirtualDeviceList(obj.Config.Hardware.Device).SelectByType((*types.VirtualEthernetCard)(nil))
	if len(devices) == 0 {
		return nil
	}

	out := make([]manager.VMNetworkAdapter, 0, len(devices))
	for idx, device := range devices {
		card, ok := device.(types.BaseVirtualEthernetCard)
		if !ok {
			continue
		}

		ethernet := card.GetVirtualEthernetCard()
		macAddress := strings.ToLower(strings.TrimSpace(ethernet.MacAddress))
		guestDetails := guestNICs[macAddress]
		networkID, networkName, networkType := describeVCenterNetworkBacking(ethernet.Backing, distributedPortgroupsByKey)
		if guestDetails.network != "" {
			networkName = guestDetails.network
		}

		label := fmt.Sprintf("Network adapter %d", idx+1)
		if ethernet.DeviceInfo != nil {
			if desc := ethernet.DeviceInfo.GetDescription(); desc != nil && desc.Label != "" {
				label = desc.Label
			}
		}

		connected := true
		if ethernet.Connectable != nil {
			connected = ethernet.Connectable.Connected
		}

		out = append(out, manager.VMNetworkAdapter{
			ID:          fmt.Sprint(ethernet.Key),
			Label:       label,
			NetworkID:   networkID,
			Network:     networkName,
			NetworkType: networkType,
			MACAddress:  macAddress,
			Connected:   connected,
			IPAddresses: append([]string(nil), guestDetails.ipAddresses...),
		})
	}

	return out
}

func vCenterGuestNICDetailsByMAC(obj mo.VirtualMachine) map[string]vCenterGuestNICDetails {
	if obj.Guest == nil || len(obj.Guest.Net) == 0 {
		return nil
	}

	out := make(map[string]vCenterGuestNICDetails, len(obj.Guest.Net))
	for _, nic := range obj.Guest.Net {
		macAddress := strings.ToLower(strings.TrimSpace(nic.MacAddress))
		if macAddress == "" {
			continue
		}

		details := out[macAddress]
		if details.network == "" {
			details.network = nic.Network
		}
		for _, ipAddress := range guestNICIPAddresses(nic) {
			details.ipAddresses = manager.AppendUnique(details.ipAddresses, ipAddress)
		}
		out[macAddress] = details
	}

	return out
}

func guestNICIPAddresses(nic types.GuestNicInfo) []string {
	var out []string
	if nic.IpConfig != nil {
		for _, ip := range nic.IpConfig.IpAddress {
			if strings.TrimSpace(ip.IpAddress) != "" {
				out = append(out, ip.IpAddress)
			}
		}
	}
	if len(out) == 0 {
		for _, ipAddress := range nic.IpAddress {
			if strings.TrimSpace(ipAddress) != "" {
				out = append(out, ipAddress)
			}
		}
	}
	return out
}

func describeVCenterNetworkBacking(backing types.BaseVirtualDeviceBackingInfo, distributedPortgroupsByKey map[string]vCenterDistributedPortgroupRef) (string, string, string) {
	switch typed := backing.(type) {
	case *types.VirtualEthernetCardNetworkBackingInfo:
		id := ""
		if typed.Network != nil {
			id = vCenterNetworkOptionID(*typed.Network)
		}
		return id, typed.DeviceName, "Standard"
	case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
		portgroup := distributedPortgroupsByKey[typed.Port.PortgroupKey]
		name := "Distributed port group"
		id := ""
		if portgroup.Ref.Type != "" {
			id = vCenterNetworkOptionID(portgroup.Ref)
		}
		if portgroup.Name != "" {
			name = portgroup.Name
		}
		return id, name, "Distributed"
	case *types.VirtualEthernetCardOpaqueNetworkBackingInfo:
		return "", typed.OpaqueNetworkId, "Opaque"
	default:
		return "", "", ""
	}
}

func vmParentRef(obj mo.VirtualMachine) *types.ManagedObjectReference {
	if obj.ParentVApp != nil {
		return obj.ParentVApp
	}
	return obj.Parent
}

func pathKey(ref *types.ManagedObjectReference) string {
	if ref == nil {
		return ""
	}
	return ref.Type + ":" + ref.Value
}

func clonePathSegments(segments []string) []string {
	if len(segments) == 0 {
		return nil
	}
	return append([]string(nil), segments...)
}

func (b *Backend) cachedInventoryPathSegments(ref *types.ManagedObjectReference) ([]string, bool) {
	key := pathKey(ref)
	if key == "" {
		return nil, true
	}

	now := time.Now()
	b.pathCacheMu.RLock()
	entry, ok := b.pathCache[key]
	b.pathCacheMu.RUnlock()
	if !ok || now.After(entry.expiresAt) {
		return nil, false
	}
	return clonePathSegments(entry.segments), true
}

func (b *Backend) storeInventoryPathSegments(ref *types.ManagedObjectReference, segments []string) {
	key := pathKey(ref)
	if key == "" {
		return
	}

	b.pathCacheMu.Lock()
	b.pathCache[key] = cachedPathSegments{
		segments:  clonePathSegments(segments),
		expiresAt: time.Now().Add(inventoryPathCacheTTL),
	}
	b.pathCacheMu.Unlock()
}

func (b *Backend) inventoryPathSegments(ctx context.Context, client *govmomi.Client, ref *types.ManagedObjectReference) ([]string, error) {
	if ref == nil {
		return nil, nil
	}
	if segments, ok := b.cachedInventoryPathSegments(ref); ok {
		return segments, nil
	}

	path, err := find.InventoryPath(ctx, client.Client, *ref)
	if err != nil {
		return nil, fmt.Errorf("resolving inventory path: %w", err)
	}
	segments := normalizeInventoryPathSegments(path)
	b.storeInventoryPathSegments(ref, segments)
	return segments, nil
}

func (b *Backend) resolveVMParentPathSegments(ctx context.Context, client *govmomi.Client, vms []mo.VirtualMachine) (map[string][]string, error) {
	out := make(map[string][]string, len(vms))
	for _, obj := range vms {
		parent := vmParentRef(obj)
		key := pathKey(parent)
		if key == "" {
			continue
		}
		if _, ok := out[key]; ok {
			continue
		}
		segments, err := b.inventoryPathSegments(ctx, client, parent)
		if err != nil {
			return nil, err
		}
		out[key] = segments
	}
	return out, nil
}

func normalizeInventoryPathSegments(path string) []string {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return nil
	}

	segments := strings.Split(trimmed, "/")
	if len(segments) >= 2 && segments[1] == "vm" {
		segments = append(segments[:1], segments[2:]...)
	}
	return segments
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
		"name",
		"config.name", "config.guestFullName",
		"config.hardware.numCPU", "config.hardware.memoryMB",
		"config.uuid", "config.version", "config.firmware", "config.annotation",
		"config.hardware.device",
		"runtime.powerState", "runtime.host",
		"guest.toolsStatus", "guest.guestOperationsReady", "guest.ipAddress", "guest.hostName", "guest.net",
		"datastore",
		"parent", "parentVApp",
	}, &obj); err != nil {
		return manager.VMInfo{}, fmt.Errorf("reading VM properties: %w", err)
	}

	client, err := b.session.Client()
	if err != nil {
		return manager.VMInfo{}, err
	}

	pathSegments, err := b.inventoryPathSegments(ctx, client, vmParentRef(obj))
	if err != nil {
		return manager.VMInfo{}, err
	}

	var distributedPortgroupsByKey map[string]vCenterDistributedPortgroupRef
	if obj.Config != nil && hasDistributedVCenterAdapter(obj.Config.Hardware.Device) {
		distributedPortgroupsByKey, _ = b.distributedPortgroupsByKey(ctx)
	}
	info := toVMInfo(obj, pathSegments, distributedPortgroupsByKey)
	if obj.Runtime.Host != nil {
		if hostName, err := object.NewHostSystem(client.Client, *obj.Runtime.Host).ObjectName(ctx); err == nil {
			info.HostName = hostName
		}
	}
	for _, datastoreRef := range obj.Datastore {
		if datastoreName, err := object.NewDatastore(client.Client, datastoreRef).ObjectName(ctx); err == nil && datastoreName != "" {
			info.DatastoreNames = manager.AppendUnique(info.DatastoreNames, datastoreName)
		}
	}

	return info, nil
}

func (b *Backend) UpdateVMConfig(ctx context.Context, emit jobs.EmitFn, req manager.VMConfigUpdateRequest) error {
	emit(10, "Loading current VM configuration...")
	info, err := b.GetVM(ctx, req.VMRef)
	if err != nil {
		return err
	}

	nextName := strings.TrimSpace(req.Name)
	nextNotes := strings.TrimSpace(req.Notes)
	currentNotes := strings.TrimSpace(info.Notes)
	requestedFirmware := normalizeVCenterFirmware(req.Firmware)
	currentFirmware := normalizeVCenterFirmware(info.Firmware)

	renameNeeded := nextName != "" && nextName != info.Name
	notesChanged := nextNotes != currentNotes
	hardwareChanged := req.NumCPU != info.NumCPU || req.MemoryMB != info.MemoryMB || requestedFirmware != currentFirmware

	if hardwareChanged && info.PowerState != "poweredOff" {
		return fmt.Errorf("CPU, memory, and firmware changes require the VM to be powered off")
	}
	if !renameNeeded && !notesChanged && !hardwareChanged {
		emit(100, "Configuration already matches the requested values.")
		return nil
	}

	vm, err := b.vmObject(ctx, req.VMRef)
	if err != nil {
		return err
	}

	if renameNeeded {
		emit(35, "Renaming VM...")
		task, err := vm.Rename(ctx, nextName)
		if err != nil {
			return fmt.Errorf("renaming VM: %w", err)
		}
		if err := task.Wait(ctx); err != nil {
			return fmt.Errorf("waiting for rename: %w", err)
		}
	}

	if notesChanged || hardwareChanged {
		spec := types.VirtualMachineConfigSpec{}
		if notesChanged {
			spec.Annotation = nextNotes
		}
		if req.NumCPU != info.NumCPU {
			spec.NumCPUs = req.NumCPU
		}
		if req.MemoryMB != info.MemoryMB {
			spec.MemoryMB = int64(req.MemoryMB)
		}
		if requestedFirmware != currentFirmware {
			spec.Firmware = requestedFirmware
		}

		emit(75, "Applying VM configuration changes...")
		task, err := vm.Reconfigure(ctx, spec)
		if err != nil {
			return fmt.Errorf("reconfiguring VM: %w", err)
		}
		if err := task.Wait(ctx); err != nil {
			return fmt.Errorf("waiting for reconfigure: %w", err)
		}
	}

	emit(100, "Configuration updated.")
	return nil
}

func (b *Backend) ConsoleInfo(ctx context.Context, vmRef string) (manager.ConsoleLaunchInfo, error) {
	client, err := b.session.Client()
	if err != nil {
		return manager.ConsoleLaunchInfo{}, err
	}

	vm, err := b.vmObject(ctx, vmRef)
	if err != nil {
		return manager.ConsoleLaunchInfo{}, err
	}

	vmName, err := vm.ObjectName(ctx)
	if err != nil {
		return manager.ConsoleLaunchInfo{}, fmt.Errorf("retrieving VM name for console: %w", err)
	}
	if vmName == "" {
		vmName = vmRef
	}

	cloneTicket, err := session.NewManager(client.Client).AcquireCloneTicket(ctx)
	if err != nil {
		return manager.ConsoleLaunchInfo{}, fmt.Errorf("acquiring console session ticket: %w", err)
	}

	baseURL := client.Client.URL()
	if baseURL == nil {
		return manager.ConsoleLaunchInfo{}, fmt.Errorf("determining vCenter URL for console")
	}
	consoleURL := *baseURL
	consoleURL.User = nil

	reportedFQDN, _ := b.reportedVCenterFQDN(ctx, client)
	consoleHost, hostSource, warnings, err := b.consoleHost(&consoleURL, reportedFQDN)
	if err != nil {
		return manager.ConsoleLaunchInfo{}, err
	}

	thumbprint := ""
	if strings.EqualFold(consoleURL.Scheme, "https") {
		var certInfo object.HostCertificateInfo
		if err := certInfo.FromURL(&consoleURL, nil); err != nil {
			return manager.ConsoleLaunchInfo{}, fmt.Errorf("retrieving vCenter certificate thumbprint: %w", err)
		}
		thumbprint = certInfo.ThumbprintSHA1
	}

	consoleURL.Path = "/ui/webconsole.html"
	consoleURL.RawQuery = url.Values{
		"vmId":          []string{vm.Reference().Value},
		"vmName":        []string{vmName},
		"serverGuid":    []string{client.ServiceContent.About.InstanceUuid},
		"host":          []string{consoleHost},
		"sessionTicket": []string{cloneTicket},
		"thumbprint":    []string{thumbprint},
	}.Encode()

	return manager.ConsoleLaunchInfo{
		URL:               consoleURL.String(),
		VMRef:             vmRef,
		VMID:              vm.Reference().Value,
		VMName:            vmName,
		ServerGUID:        client.ServiceContent.About.InstanceUuid,
		VCenterURL:        consoleURL.Scheme + "://" + consoleURL.Host,
		ConnectedHost:     strings.TrimSpace(consoleURL.Hostname()),
		ReportedFQDN:      reportedFQDN,
		ConsoleHost:       consoleHost,
		ConsoleHostSource: hostSource,
		Thumbprint:        thumbprint,
		TicketPreview:     ticketPreview(cloneTicket),
		Warnings:          warnings,
	}, nil
}

func (b *Backend) reportedVCenterFQDN(ctx context.Context, client *govmomi.Client) (string, error) {
	if client.ServiceContent.Setting != nil {
		optionManager := object.NewOptionManager(client.Client, *client.ServiceContent.Setting)
		if values, err := optionManager.Query(ctx, "VirtualCenter.FQDN"); err == nil && len(values) > 0 {
			if optionValue := values[0].GetOptionValue(); optionValue != nil {
				if value, ok := optionValue.Value.(string); ok {
					if value = strings.TrimSpace(value); value != "" {
						return value, nil
					}
				}
			}
		}
	}

	return "", nil
}

func (b *Backend) consoleHost(baseURL *url.URL, reportedFQDN string) (string, string, []string, error) {
	var warnings []string

	connectedHost := ""
	if baseURL != nil {
		connectedHost = strings.TrimSpace(baseURL.Hostname())
	}

	if reportedFQDN != "" {
		if connectedHost != "" && !strings.EqualFold(reportedFQDN, connectedHost) {
			warnings = append(warnings, fmt.Sprintf(
				"vCenter reports FQDN %q while the current session is connected to %q; the console link is using the vCenter-reported host.",
				reportedFQDN,
				connectedHost,
			))
		}
		return reportedFQDN, consoleHostSourceReportedFQDN, warnings, nil
	}

	if connectedHost != "" {
		return connectedHost, consoleHostSourceConnectedHost, warnings, nil
	}

	if host := strings.TrimSpace((&url.URL{Host: b.session.Host()}).Hostname()); host != "" {
		warnings = append(warnings, "xman could not recover the original connection hostname, so it fell back to the stored session host.")
		return host, consoleHostSourceSessionHost, warnings, nil
	}

	return "", "", warnings, fmt.Errorf("determining vCenter console host")
}

func ticketPreview(ticket string) string {
	ticket = strings.TrimSpace(ticket)
	if len(ticket) <= 12 {
		return ticket
	}
	return ticket[:6] + "..." + ticket[len(ticket)-4:]
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
		return wrapGuestOpsError("uploading file", err)
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
		return wrapGuestOpsError("downloading file", err)
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
		return wrapGuestOpsError("starting command", err)
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

	output := executil.NormalizeCapturedOutput(data)
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
		return nil, wrapGuestOpsError("creating guest toolbox client", err)
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
			return -1, wrapGuestOpsError("checking process status", err)
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
			lastErr = wrapGuestOpsError("downloading output", err)
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

func terminateGuestProcess(tools *toolbox.Client, pid int64) {
	killCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = tools.ProcessManager.TerminateProcess(killCtx, tools.Authentication, pid)
}

type toolsInstallState struct {
	guestOS               string
	toolsStatus           types.VirtualMachineToolsStatus
	poweredOn             bool
	hasGuest              bool
	guestOpsReady         bool
	toolsInstallerMounted bool
}

func readToolsInstallState(ctx context.Context, vm *object.VirtualMachine) (toolsInstallState, error) {
	var obj mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{
		"runtime.powerState",
		"runtime.toolsInstallerMounted",
		"guest.toolsStatus",
		"guest.guestOperationsReady",
		"config.guestFullName",
	}, &obj); err != nil {
		return toolsInstallState{}, fmt.Errorf("reading VM properties: %w", err)
	}

	state := toolsInstallState{
		poweredOn:             obj.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn,
		hasGuest:              obj.Guest != nil,
		toolsInstallerMounted: obj.Runtime.ToolsInstallerMounted,
	}
	if obj.Config != nil {
		state.guestOS = obj.Config.GuestFullName
	}
	if obj.Guest != nil {
		state.toolsStatus = obj.Guest.ToolsStatus
		if obj.Guest.GuestOperationsReady != nil {
			state.guestOpsReady = *obj.Guest.GuestOperationsReady
		}
	}

	return state, nil
}

func validateToolsInstallState(state toolsInstallState) error {
	if !state.poweredOn {
		return fmt.Errorf("VM must be powered on to install VMware Tools")
	}
	return nil
}

func mountToolsInstaller(ctx context.Context, emit jobs.EmitFn, vm *object.VirtualMachine) error {
	emit(10, "Mounting VMware Tools installer...")
	if err := vm.MountToolsInstaller(ctx); err != nil {
		return fmt.Errorf("mounting tools installer: %w", err)
	}
	emit(35, "VMware Tools installer mounted.")
	return nil
}

func upgradeTools(ctx context.Context, emit jobs.EmitFn, vm *object.VirtualMachine) error {
	emit(10, "Requesting VMware Tools upgrade from vSphere...")
	task, err := vm.UpgradeTools(ctx, "")
	if err != nil {
		return fmt.Errorf("requesting VMware Tools upgrade: %w", err)
	}

	emit(50, "Installing VMware Tools, this may take a few minutes...")
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("VMware Tools installation task failed: %w", err)
	}
	return nil
}

func waitForGuestOperationsReady(ctx context.Context, emit jobs.EmitFn, vm *object.VirtualMachine, timeout time.Duration, progress int) (toolsInstallState, bool, error) {
	ticker := time.NewTicker(guestOpsReadyPollInterval)
	timer := time.NewTimer(timeout)
	defer ticker.Stop()
	defer timer.Stop()

	var lastState toolsInstallState
	for {
		state, err := readToolsInstallState(ctx, vm)
		if err != nil {
			return state, false, err
		}
		lastState = state
		if state.guestOpsReady {
			return state, true, nil
		}

		message := "Waiting for VMware guest operations to become ready..."
		if state.toolsInstallerMounted {
			message = "Waiting for VMware Tools to finish starting inside the guest..."
		} else if state.toolsStatus == types.VirtualMachineToolsStatusToolsNotInstalled {
			message = "Waiting for VMware Tools installation to start in the guest..."
		}
		emit(progress, message)

		select {
		case <-ctx.Done():
			return lastState, false, ctx.Err()
		case <-timer.C:
			return lastState, false, nil
		case <-ticker.C:
		}
	}
}

func fallbackGuestOpsMessage(state toolsInstallState) string {
	if manager.IsWindows(state.guestOS) {
		return "VMware Tools installer is mounted. Windows may still be starting setup in the guest; xman will enable Guest Ops automatically as soon as vSphere reports readiness."
	}
	return "VMware Tools installer is mounted. Complete the install inside the guest, and xman will enable Guest Ops automatically once vSphere reports readiness."
}

func wrapGuestOpsError(action string, err error) error {
	switch {
	case fault.Is(err, &types.ToolsUnavailable{}), fault.Is(err, &types.GuestOperationsUnavailable{}):
		return fmt.Errorf("%s: VMware guest operations are not ready yet. If the VM just booted, wait a bit and try again. If this is a fresh VM, use Bootstrap Guest Ops in VM Info", action)
	case fault.Is(err, &types.InvalidPowerState{}), fault.Is(err, &types.InvalidState{}):
		return fmt.Errorf("%s: the VM must be powered on and responsive for VMware guest operations", action)
	default:
		return fmt.Errorf("%s: %w", action, err)
	}
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

	if state.guestOpsReady && state.toolsStatus != types.VirtualMachineToolsStatusToolsOld {
		emit(100, "Guest operations are already ready.")
		return nil
	}

	if state.toolsStatus != types.VirtualMachineToolsStatusToolsNotInstalled && !state.guestOpsReady {
		if state.toolsStatus == types.VirtualMachineToolsStatusToolsNotRunning {
			emit(15, "VMware Tools are installed but still starting...")
		} else {
			emit(15, "VMware Tools look installed, but guest operations are still warming up...")
		}
		warmState, ready, err := waitForGuestOperationsReady(ctx, emit, vm, guestOpsWarmupWait, 45)
		if err != nil {
			return err
		}
		if ready {
			emit(100, "Guest operations are ready.")
			return nil
		}
		state = warmState
		if state.toolsStatus == types.VirtualMachineToolsStatusToolsOk {
			emit(100, "VMware Tools are installed, but guest operations are still starting. xman will enable them automatically once vSphere reports readiness.")
			return nil
		}
	}

	if !state.hasGuest || state.toolsStatus == types.VirtualMachineToolsStatusToolsNotInstalled {
		if !state.toolsInstallerMounted {
			if err := mountToolsInstaller(ctx, emit, vm); err != nil {
				return err
			}
		} else {
			emit(20, "VMware Tools installer is already mounted.")
		}

		finalState, ready, err := waitForGuestOperationsReady(ctx, emit, vm, guestOpsBootstrapWait, 65)
		if err != nil {
			return err
		}
		if ready {
			emit(100, "Guest operations are ready.")
			return nil
		}

		emit(100, fallbackGuestOpsMessage(finalState))
		return nil
	}

	if err := upgradeTools(ctx, emit, vm); err != nil {
		if manager.IsWindows(state.guestOS) {
			emit(55, "Automatic VMware Tools install/upgrade was not available - mounting the installer instead...")
			if !state.toolsInstallerMounted {
				if mountErr := mountToolsInstaller(ctx, emit, vm); mountErr != nil {
					return fmt.Errorf("%w; mount fallback failed: %v", err, mountErr)
				}
			}
			finalState, ready, waitErr := waitForGuestOperationsReady(ctx, emit, vm, guestOpsBootstrapWait, 70)
			if waitErr != nil {
				return waitErr
			}
			if ready {
				emit(100, "Guest operations are ready.")
				return nil
			}
			emit(100, fallbackGuestOpsMessage(finalState))
			return nil
		}
		return err
	}

	finalState, ready, err := waitForGuestOperationsReady(ctx, emit, vm, guestOpsUpgradeWait, 75)
	if err != nil {
		return err
	}
	if ready {
		emit(100, "Guest operations are ready.")
		return nil
	}

	if finalState.toolsInstallerMounted {
		emit(100, fallbackGuestOpsMessage(finalState))
		return nil
	}

	emit(100, "VMware Tools install/upgrade was requested. Guest operations are still starting, and xman will enable them automatically once vSphere reports readiness.")
	return nil
}
