package snapshots

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"manosphere/internal/jobs"
	"manosphere/internal/vcenter"
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

// Service provides VM snapshot operations against vCenter.
type Service struct {
	session *vcenter.Session
}

// NewService creates a snapshots Service.
func NewService(s *vcenter.Session) *Service {
	return &Service{session: s}
}

func (s *Service) vmObject(vmRef string) (*object.VirtualMachine, error) {
	client, err := s.session.Client()
	if err != nil {
		return nil, err
	}
	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: vmRef}
	return object.NewVirtualMachine(client.Client, ref), nil
}

func (s *Service) snapMOR(snapRef string) types.ManagedObjectReference {
	return types.ManagedObjectReference{Type: "VirtualMachineSnapshot", Value: snapRef}
}

// ListSnapshots returns a flat list of all snapshots for a VM.
func (s *Service) ListSnapshots(ctx context.Context, vmRef string) ([]SnapshotInfo, error) {
	vm, err := s.vmObject(vmRef)
	if err != nil {
		return nil, err
	}

	var obj mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"snapshot"}, &obj); err != nil {
		return nil, fmt.Errorf("reading snapshot properties: %w", err)
	}

	if obj.Snapshot == nil {
		return []SnapshotInfo{}, nil
	}

	var currentRef string
	if obj.Snapshot.CurrentSnapshot != nil {
		currentRef = obj.Snapshot.CurrentSnapshot.Value
	}

	var out []SnapshotInfo
	var walk func(nodes []types.VirtualMachineSnapshotTree, depth int)
	walk = func(nodes []types.VirtualMachineSnapshotTree, depth int) {
		for _, node := range nodes {
			out = append(out, SnapshotInfo{
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

// CreateSnapshot creates a new snapshot and returns a job ID.
func (s *Service) CreateSnapshot(ctx context.Context, emit jobs.EmitFn, vmRef, name, description string, memory, quiesce bool) error {
	vm, err := s.vmObject(vmRef)
	if err != nil {
		return err
	}
	emit(10, "Creating snapshot...")
	task, err := vm.CreateSnapshot(ctx, name, description, memory, quiesce)
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("create snapshot task: %w", err)
	}
	emit(100, fmt.Sprintf("Snapshot %q created", name))
	return nil
}

// RevertSnapshot reverts the VM to the given snapshot.
func (s *Service) RevertSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string) error {
	client, err := s.session.Client()
	if err != nil {
		return err
	}
	emit(10, "Reverting to snapshot...")
	res, err := methods.RevertToSnapshot_Task(ctx, client.Client, &types.RevertToSnapshot_Task{
		This: s.snapMOR(snapRef),
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

// DeleteSnapshot deletes the given snapshot.
func (s *Service) DeleteSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string, removeChildren bool) error {
	client, err := s.session.Client()
	if err != nil {
		return err
	}
	emit(10, "Deleting snapshot...")
	res, err := methods.RemoveSnapshot_Task(ctx, client.Client, &types.RemoveSnapshot_Task{
		This:           s.snapMOR(snapRef),
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
