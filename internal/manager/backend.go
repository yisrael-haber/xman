package manager

import (
	"context"

	"xman/internal/jobs"
)

// Capabilities describes which optional feature groups a backend supports.
type Capabilities struct {
	GuestOps     bool // Upload, Download, GuestRun
	Inventory    bool // ListHosts, ListDatastores
	ToolsInstall bool // InstallTools
	Console      bool // Web console launch available
}

type EndpointCheck struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

type ConsoleLaunchInfo struct {
	URL               string        `json:"url"`
	DisplayURL        string        `json:"displayUrl"`
	VMRef             string        `json:"vmRef"`
	VMID              string        `json:"vmId"`
	VMName            string        `json:"vmName"`
	ServerGUID        string        `json:"serverGuid"`
	VCenterURL        string        `json:"vcenterUrl"`
	ConnectedHost     string        `json:"connectedHost"`
	ReportedFQDN      string        `json:"reportedFqdn"`
	ConsoleHost       string        `json:"consoleHost"`
	ConsoleHostSource string        `json:"consoleHostSource"`
	Thumbprint        string        `json:"thumbprint,omitempty"`
	TicketPreview     string        `json:"ticketPreview"`
	Warnings          []string      `json:"warnings"`
	VCenterCheck      EndpointCheck `json:"vcenterCheck"`
	ConsoleHostCheck  EndpointCheck `json:"consoleHostCheck"`
}

// Backend is the interface all hypervisor backends must implement.
// The Manager holds one active Backend between Connect and Disconnect.
type Backend interface {
	BackendType() string
	DisplayName() string
	Capabilities() Capabilities
	Disconnect(ctx context.Context) error

	// VM lifecycle
	ListVMs(ctx context.Context) ([]VMInfo, error)
	GetVM(ctx context.Context, vmRef string) (VMInfo, error)
	PowerOn(ctx context.Context, vmRef string) error
	PowerOff(ctx context.Context, vmRef string) error
	Reset(ctx context.Context, vmRef string) error
	Suspend(ctx context.Context, vmRef string) error
	UpdateVMConfig(ctx context.Context, emit jobs.EmitFn, req VMConfigUpdateRequest) error
	ListVMNetworkOptions(ctx context.Context, vmRef string) ([]VMNetworkOption, error)
	UpdateVMNetwork(ctx context.Context, emit jobs.EmitFn, req VMNetworkUpdateRequest) error

	// Snapshots
	ListSnapshots(ctx context.Context, vmRef string) ([]SnapshotInfo, error)
	CreateSnapshot(ctx context.Context, emit jobs.EmitFn, req CreateSnapshotRequest) error
	RevertSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string) error
	DeleteSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string, removeChildren bool) error

	// Networks are available across current backends.
	ListNetworks(ctx context.Context) (NetworkSummary, error)
}

// GuestOpsBackend is implemented by backends that support guest command and file operations.
type GuestOpsBackend interface {
	Upload(ctx context.Context, emit jobs.EmitFn, req UploadRequest) error
	Download(ctx context.Context, emit jobs.EmitFn, req DownloadRequest) error
	GuestRun(ctx context.Context, emit jobs.EmitFn, req RunRequest) error
}

// InventoryBackend is implemented by backends that support host and datastore inventory.
type InventoryBackend interface {
	ListHosts(ctx context.Context) ([]HostInfo, error)
	ListDatastores(ctx context.Context) ([]DatastoreInfo, error)
}

// ToolsInstallBackend is implemented by backends that support VMware Tools install flows.
type ToolsInstallBackend interface {
	InstallTools(ctx context.Context, emit jobs.EmitFn, vmRef string) error
}

// ConsoleBackend is implemented by backends that support browser console launch info.
type ConsoleBackend interface {
	ConsoleInfo(ctx context.Context, vmRef string) (ConsoleLaunchInfo, error)
}
