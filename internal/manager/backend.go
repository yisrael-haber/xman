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
}

// Backend is the interface all hypervisor backends must implement.
// The Manager holds one active Backend between Connect and Disconnect.
type Backend interface {
	DisplayName() string
	Capabilities() Capabilities
	Disconnect(ctx context.Context) error

	// VM lifecycle
	ListVMs(ctx context.Context) ([]VMInfo, error)
	PowerOn(ctx context.Context, vmRef string) error
	PowerOff(ctx context.Context, vmRef string) error
	Reset(ctx context.Context, vmRef string) error
	Suspend(ctx context.Context, vmRef string) error

	// Snapshots
	ListSnapshots(ctx context.Context, vmRef string) ([]SnapshotInfo, error)
	CreateSnapshot(ctx context.Context, emit jobs.EmitFn, req CreateSnapshotRequest) error
	RevertSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string) error
	DeleteSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string, removeChildren bool) error

	// Guest operations (only if Capabilities.GuestOps == true)
	Upload(ctx context.Context, emit jobs.EmitFn, req UploadRequest) error
	Download(ctx context.Context, emit jobs.EmitFn, req DownloadRequest) error
	GuestRun(ctx context.Context, emit jobs.EmitFn, req RunRequest) error

	// Inventory (only if Capabilities.Inventory == true)
	ListHosts(ctx context.Context) ([]HostInfo, error)
	ListDatastores(ctx context.Context) ([]DatastoreInfo, error)

	// Tools (only if Capabilities.ToolsInstall == true)
	InstallTools(ctx context.Context, emit jobs.EmitFn, vmRef string) error
}
