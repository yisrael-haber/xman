package manager

import (
	"context"
	"time"

	"xman/internal/jobs"
)

// SnapshotInfo is a serialisable summary of a VM snapshot.
type SnapshotInfo struct {
	Ref         string    `json:"ref"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreateTime  time.Time `json:"createTime"`
	IsCurrent   bool      `json:"isCurrent"`
	Depth       int       `json:"depth"`
}

// CreateSnapshotRequest is the payload to create a new snapshot.
type CreateSnapshotRequest struct {
	VMRef       string `json:"vmRef"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Memory      bool   `json:"memory"`
	Quiesce     bool   `json:"quiesce"`
}

func (m *Manager) SnapshotList(vmRef string) ([]SnapshotInfo, error) {
	b, err := m.getBackend()
	if err != nil {
		return nil, err
	}
	return b.ListSnapshots(m.operationContext(), vmRef)
}

func (m *Manager) SnapshotCreate(req CreateSnapshotRequest) string {
	return m.submitJob("snapshots", "Snapshot: "+req.Name, func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		return b.CreateSnapshot(ctx, emit, req)
	})
}

func (m *Manager) SnapshotRevert(snapRef string) string {
	return m.submitJob("snapshots", "Revert to snapshot", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		return b.RevertSnapshot(ctx, emit, snapRef)
	})
}

func (m *Manager) SnapshotDelete(snapRef string, removeChildren bool) string {
	return m.submitJob("snapshots", "Delete snapshot", func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		return b.DeleteSnapshot(ctx, emit, snapRef, removeChildren)
	})
}
