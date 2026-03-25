package snapshots

import (
	"context"

	"manosphere/internal/jobs"
	"manosphere/internal/vcenter"
)

// CreateRequest is the payload to create a new snapshot.
type CreateRequest struct {
	VMRef       string `json:"vmRef"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Memory      bool   `json:"memory"`
	Quiesce     bool   `json:"quiesce"`
}

// Binding exposes snapshot operations to the Wails frontend.
type Binding struct {
	service *Service
	jobs    *jobs.Manager
	ctx     context.Context
}

// NewBinding creates a Binding backed by the given vCenter session and job manager.
func NewBinding(s *vcenter.Session, jm *jobs.Manager) *Binding {
	return &Binding{service: NewService(s), jobs: jm}
}

// SetContext is called by app.go on startup to provide the Wails runtime context.
func (b *Binding) SetContext(ctx context.Context) {
	b.ctx = ctx
}

// SnapshotList returns all snapshots for a VM.
func (b *Binding) SnapshotList(vmRef string) ([]SnapshotInfo, error) {
	return b.service.ListSnapshots(b.ctx, vmRef)
}

// SnapshotCreate starts a snapshot creation job and returns the job ID.
func (b *Binding) SnapshotCreate(req CreateRequest) string {
	label := "Snapshot: " + req.Name
	return b.jobs.Submit("snapshots", label, func(ctx context.Context, emit jobs.EmitFn) error {
		return b.service.CreateSnapshot(ctx, emit, req.VMRef, req.Name, req.Description, req.Memory, req.Quiesce)
	})
}

// SnapshotRevert starts a revert job and returns the job ID.
func (b *Binding) SnapshotRevert(snapRef string) string {
	label := "Revert to snapshot"
	return b.jobs.Submit("snapshots", label, func(ctx context.Context, emit jobs.EmitFn) error {
		return b.service.RevertSnapshot(ctx, emit, snapRef)
	})
}

// SnapshotDelete starts a snapshot deletion job and returns the job ID.
func (b *Binding) SnapshotDelete(snapRef string, removeChildren bool) string {
	label := "Delete snapshot"
	return b.jobs.Submit("snapshots", label, func(ctx context.Context, emit jobs.EmitFn) error {
		return b.service.DeleteSnapshot(ctx, emit, snapRef, removeChildren)
	})
}
