package manager

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"xman/internal/config"
	"xman/internal/jobs"
)

type fakeBackend struct {
	mu sync.Mutex

	backendType  string
	displayName  string
	capabilities Capabilities

	disconnectCalls int

	listVMsFunc        func(context.Context) ([]VMInfo, error)
	getVMFunc          func(context.Context, string) (VMInfo, error)
	powerOnFunc        func(context.Context, string) error
	powerOffFunc       func(context.Context, string) error
	resetFunc          func(context.Context, string) error
	suspendFunc        func(context.Context, string) error
	listSnapshotsFunc  func(context.Context, string) ([]SnapshotInfo, error)
	createSnapshotFunc func(context.Context, jobs.EmitFn, CreateSnapshotRequest) error
	revertSnapshotFunc func(context.Context, jobs.EmitFn, string) error
	deleteSnapshotFunc func(context.Context, jobs.EmitFn, string, bool) error
	uploadFunc         func(context.Context, jobs.EmitFn, UploadRequest) error
	downloadFunc       func(context.Context, jobs.EmitFn, DownloadRequest) error
	guestRunFunc       func(context.Context, jobs.EmitFn, RunRequest) error
	listHostsFunc      func(context.Context) ([]HostInfo, error)
	listDatastoresFunc func(context.Context) ([]DatastoreInfo, error)
	listNetworksFunc   func(context.Context) (NetworkSummary, error)
	installToolsFunc   func(context.Context, jobs.EmitFn, string) error
}

func (b *fakeBackend) BackendType() string        { return b.backendType }
func (b *fakeBackend) DisplayName() string        { return b.displayName }
func (b *fakeBackend) Capabilities() Capabilities { return b.capabilities }
func (b *fakeBackend) Disconnect(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.disconnectCalls++
	return nil
}
func (b *fakeBackend) ListVMs(ctx context.Context) ([]VMInfo, error) {
	if b.listVMsFunc != nil {
		return b.listVMsFunc(ctx)
	}
	return nil, errors.New("ListVMs not implemented")
}
func (b *fakeBackend) GetVM(ctx context.Context, vmRef string) (VMInfo, error) {
	if b.getVMFunc != nil {
		return b.getVMFunc(ctx, vmRef)
	}
	return VMInfo{}, errors.New("GetVM not implemented")
}
func (b *fakeBackend) PowerOn(ctx context.Context, vmRef string) error {
	if b.powerOnFunc != nil {
		return b.powerOnFunc(ctx, vmRef)
	}
	return nil
}
func (b *fakeBackend) PowerOff(ctx context.Context, vmRef string) error {
	if b.powerOffFunc != nil {
		return b.powerOffFunc(ctx, vmRef)
	}
	return nil
}
func (b *fakeBackend) Reset(ctx context.Context, vmRef string) error {
	if b.resetFunc != nil {
		return b.resetFunc(ctx, vmRef)
	}
	return nil
}
func (b *fakeBackend) Suspend(ctx context.Context, vmRef string) error {
	if b.suspendFunc != nil {
		return b.suspendFunc(ctx, vmRef)
	}
	return nil
}
func (b *fakeBackend) ListSnapshots(ctx context.Context, vmRef string) ([]SnapshotInfo, error) {
	if b.listSnapshotsFunc != nil {
		return b.listSnapshotsFunc(ctx, vmRef)
	}
	return nil, nil
}
func (b *fakeBackend) CreateSnapshot(ctx context.Context, emit jobs.EmitFn, req CreateSnapshotRequest) error {
	if b.createSnapshotFunc != nil {
		return b.createSnapshotFunc(ctx, emit, req)
	}
	return nil
}
func (b *fakeBackend) RevertSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string) error {
	if b.revertSnapshotFunc != nil {
		return b.revertSnapshotFunc(ctx, emit, snapRef)
	}
	return nil
}
func (b *fakeBackend) DeleteSnapshot(ctx context.Context, emit jobs.EmitFn, snapRef string, removeChildren bool) error {
	if b.deleteSnapshotFunc != nil {
		return b.deleteSnapshotFunc(ctx, emit, snapRef, removeChildren)
	}
	return nil
}
func (b *fakeBackend) Upload(ctx context.Context, emit jobs.EmitFn, req UploadRequest) error {
	if b.uploadFunc != nil {
		return b.uploadFunc(ctx, emit, req)
	}
	return nil
}
func (b *fakeBackend) Download(ctx context.Context, emit jobs.EmitFn, req DownloadRequest) error {
	if b.downloadFunc != nil {
		return b.downloadFunc(ctx, emit, req)
	}
	return nil
}
func (b *fakeBackend) GuestRun(ctx context.Context, emit jobs.EmitFn, req RunRequest) error {
	if b.guestRunFunc != nil {
		return b.guestRunFunc(ctx, emit, req)
	}
	return nil
}
func (b *fakeBackend) ListHosts(ctx context.Context) ([]HostInfo, error) {
	if b.listHostsFunc != nil {
		return b.listHostsFunc(ctx)
	}
	return nil, nil
}
func (b *fakeBackend) ListDatastores(ctx context.Context) ([]DatastoreInfo, error) {
	if b.listDatastoresFunc != nil {
		return b.listDatastoresFunc(ctx)
	}
	return nil, nil
}
func (b *fakeBackend) ListNetworks(ctx context.Context) (NetworkSummary, error) {
	if b.listNetworksFunc != nil {
		return b.listNetworksFunc(ctx)
	}
	return NetworkSummary{}, nil
}
func (b *fakeBackend) InstallTools(ctx context.Context, emit jobs.EmitFn, vmRef string) error {
	if b.installToolsFunc != nil {
		return b.installToolsFunc(ctx, emit, vmRef)
	}
	return nil
}

func TestConnectionInfoReflectsBackendAndDisconnectClearsIt(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)
	backend := &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		capabilities: Capabilities{
			GuestOps:     true,
			Inventory:    false,
			ToolsInstall: true,
		},
	}

	m.ReplaceBackend(context.Background(), backend)

	info := m.ConnectionInfo()
	if info.BackendType != "workstation" || info.DisplayName != "Local Workstation" {
		t.Fatalf("ConnectionInfo() = %+v, want backend metadata", info)
	}
	if !info.GuestOps || info.Inventory || !info.ToolsInstall {
		t.Fatalf("ConnectionInfo() capabilities = %+v, want GuestOps=true Inventory=false ToolsInstall=true", info)
	}

	if err := m.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	if backend.disconnectCalls != 1 {
		t.Fatalf("backend disconnectCalls = %d, want %d", backend.disconnectCalls, 1)
	}
	if got := m.ConnectionInfo(); got != (config.ConnectionInfo{}) {
		t.Fatalf("ConnectionInfo() after disconnect = %+v, want zero value", got)
	}
}

func TestReplaceBackendCancelsConnectionScopedJobsAndDisconnectsOldBackend(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)
	oldBackend := &fakeBackend{backendType: "workstation", displayName: "Old"}
	newBackend := &fakeBackend{backendType: "vcenter", displayName: "New"}

	m.ReplaceBackend(context.Background(), oldBackend)
	jobID := m.submitJob("test", "Long running", func(ctx context.Context, emit jobs.EmitFn) error {
		emit(10, "started")
		<-ctx.Done()
		return ctx.Err()
	})

	m.ReplaceBackend(context.Background(), newBackend)

	job := waitForJob(t, jm, jobID)
	if job.Status != jobs.StatusCancelled {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusCancelled)
	}
	if oldBackend.disconnectCalls != 1 {
		t.Fatalf("old backend disconnectCalls = %d, want %d", oldBackend.disconnectCalls, 1)
	}
	if newBackend.disconnectCalls != 0 {
		t.Fatalf("new backend disconnectCalls = %d, want %d", newBackend.disconnectCalls, 0)
	}
}

func TestDisconnectCancelsActiveConnectionScopedJob(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)
	backend := &fakeBackend{backendType: "workstation", displayName: "Local"}
	m.ReplaceBackend(context.Background(), backend)

	jobID := m.submitJob("test", "Disconnect me", func(ctx context.Context, emit jobs.EmitFn) error {
		emit(10, "waiting")
		<-ctx.Done()
		return ctx.Err()
	})

	if err := m.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}

	job := waitForJob(t, jm, jobID)
	if job.Status != jobs.StatusCancelled {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusCancelled)
	}
}

func TestVMPowerOffWaitsForObservedStateBeforeJobCompletes(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)

	var mu sync.Mutex
	powerOffCalls := 0
	listCallsAfterPowerOff := 0
	backend := &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		powerOffFunc: func(context.Context, string) error {
			mu.Lock()
			defer mu.Unlock()
			powerOffCalls++
			return nil
		},
		listVMsFunc: func(context.Context) ([]VMInfo, error) {
			mu.Lock()
			defer mu.Unlock()
			state := "poweredOn"
			if powerOffCalls > 0 {
				listCallsAfterPowerOff++
				if listCallsAfterPowerOff >= 2 {
					state = "poweredOff"
				}
			}
			return []VMInfo{{
				Ref:        "vm-1",
				Name:       "vm-1",
				PowerState: state,
			}}, nil
		},
	}
	m.ReplaceBackend(context.Background(), backend)

	jobID := m.VMPowerOff("vm-1")
	job := waitForJob(t, jm, jobID)

	if job.Status != jobs.StatusDone {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusDone)
	}
	if powerOffCalls != 1 {
		t.Fatalf("PowerOff call count = %d, want %d", powerOffCalls, 1)
	}
	if listCallsAfterPowerOff < 2 {
		t.Fatalf("ListVMs after PowerOff called %d times, want at least %d to prove wait loop ran", listCallsAfterPowerOff, 2)
	}

	var sawWaiting, sawComplete bool
	for _, entry := range job.Log {
		if strings.Contains(entry.Message, "Waiting for VM to report poweredOff") {
			sawWaiting = true
		}
		if entry.Message == "Power off complete" {
			sawComplete = true
		}
	}
	if !sawWaiting {
		t.Fatalf("job log = %+v, want waiting-for-state message", job.Log)
	}
	if !sawComplete {
		t.Fatalf("job log = %+v, want final completion message", job.Log)
	}
}

func TestSnapshotCreateUsesSharedJobLabelAndCompletes(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)
	backend := &fakeBackend{
		backendType: "vcenter",
		displayName: "vCenter",
		createSnapshotFunc: func(ctx context.Context, emit jobs.EmitFn, req CreateSnapshotRequest) error {
			emit(10, "Creating snapshot...")
			emit(100, "Snapshot created")
			return nil
		},
	}
	m.ReplaceBackend(context.Background(), backend)

	jobID := m.SnapshotCreate(CreateSnapshotRequest{
		VMRef: "vm-1",
		Name:  "before-change",
	})
	job := waitForJob(t, jm, jobID)

	if job.Status != jobs.StatusDone {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusDone)
	}
	if job.Label != "Snapshot: before-change" {
		t.Fatalf("job label = %q, want %q", job.Label, "Snapshot: before-change")
	}
}

func TestGuestRunJobUsesSharedLabelAndFailureSemantics(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)
	backend := &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		guestRunFunc: func(ctx context.Context, emit jobs.EmitFn, req RunRequest) error {
			emit(25, "Executing command...")
			return errors.New("guest tools unavailable")
		},
	}
	m.ReplaceBackend(context.Background(), backend)

	jobID := m.GuestRun(RunRequest{
		VMRef:    "vm-1",
		Username: "tester",
		Password: "secret",
		Command:  "hostname",
		GuestOS:  "ubuntu-64",
	})
	job := waitForJob(t, jm, jobID)

	if job.Feature != "guestexec" {
		t.Fatalf("job feature = %q, want %q", job.Feature, "guestexec")
	}
	if job.Label != "$ hostname" {
		t.Fatalf("job label = %q, want %q", job.Label, "$ hostname")
	}
	if job.Status != jobs.StatusFailed {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusFailed)
	}
	if !strings.Contains(job.Error, "guest tools unavailable") {
		t.Fatalf("job error = %q, want guest run failure", job.Error)
	}
}

func TestUploadAndDownloadJobsUseExpectedLabels(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)

	var uploadReqs []UploadRequest
	var downloadReqs []DownloadRequest
	backend := &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		uploadFunc: func(ctx context.Context, emit jobs.EmitFn, req UploadRequest) error {
			uploadReqs = append(uploadReqs, req)
			emit(100, "Upload complete.")
			return nil
		},
		downloadFunc: func(ctx context.Context, emit jobs.EmitFn, req DownloadRequest) error {
			downloadReqs = append(downloadReqs, req)
			emit(100, "Download complete.")
			return nil
		},
	}
	m.ReplaceBackend(context.Background(), backend)

	uploadID := m.Upload(UploadRequest{
		VMRef:     "vm-1",
		Username:  "tester",
		Password:  "secret",
		LocalPath: "/tmp/local.iso",
		GuestPath: "/tmp/guest.iso",
		GuestOS:   "ubuntu-64",
	})
	downloadID := m.Download(DownloadRequest{
		VMRef:     "vm-1",
		Username:  "tester",
		Password:  "secret",
		GuestPath: "/tmp/guest.log",
		LocalPath: "/tmp/guest.log",
	})

	uploadJob := waitForJob(t, jm, uploadID)
	downloadJob := waitForJob(t, jm, downloadID)

	if uploadJob.Label != "Upload: /tmp/local.iso → /tmp/guest.iso" {
		t.Fatalf("upload label = %q", uploadJob.Label)
	}
	if downloadJob.Label != "Download: /tmp/guest.log → /tmp/guest.log" {
		t.Fatalf("download label = %q", downloadJob.Label)
	}
	if len(uploadReqs) != 1 || uploadReqs[0].GuestPath != "/tmp/guest.iso" {
		t.Fatalf("upload requests = %+v, want one delegated upload", uploadReqs)
	}
	if len(downloadReqs) != 1 || downloadReqs[0].GuestPath != "/tmp/guest.log" {
		t.Fatalf("download requests = %+v, want one delegated download", downloadReqs)
	}
}

func TestInstallAutoCommandUploadsRunsAndCleansUp(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)

	var uploads []UploadRequest
	var guestRuns []RunRequest
	backend := &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		uploadFunc: func(ctx context.Context, emit jobs.EmitFn, req UploadRequest) error {
			uploads = append(uploads, req)
			emit(100, "Upload complete.")
			return nil
		},
		guestRunFunc: func(ctx context.Context, emit jobs.EmitFn, req RunRequest) error {
			guestRuns = append(guestRuns, req)
			if strings.HasPrefix(req.Command, "rm -f ") {
				return nil
			}
			emit(100, "Command completed.")
			return nil
		},
	}
	m.ReplaceBackend(context.Background(), backend)

	jobID := m.Install(InstallRequest{
		VMRef:     "vm-1",
		Username:  "tester",
		Password:  "secret",
		LocalPath: "/tmp/agent.deb",
		GuestOS:   "ubuntu-64",
	})
	job := waitForJob(t, jm, jobID)

	if job.Status != jobs.StatusDone {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusDone)
	}
	if job.Label != "Install: agent.deb" {
		t.Fatalf("job label = %q, want %q", job.Label, "Install: agent.deb")
	}
	if len(uploads) != 1 {
		t.Fatalf("upload calls = %d, want %d", len(uploads), 1)
	}
	if uploads[0].GuestPath != "/tmp/agent.deb" {
		t.Fatalf("upload guest path = %q, want %q", uploads[0].GuestPath, "/tmp/agent.deb")
	}
	if len(guestRuns) != 2 {
		t.Fatalf("guest run calls = %d, want install + cleanup", len(guestRuns))
	}
	if !strings.Contains(guestRuns[0].Command, `dpkg -i "/tmp/agent.deb"`) {
		t.Fatalf("install command = %q, want deb install command", guestRuns[0].Command)
	}
	if guestRuns[1].Command != `rm -f "/tmp/agent.deb"` {
		t.Fatalf("cleanup command = %q, want %q", guestRuns[1].Command, `rm -f "/tmp/agent.deb"`)
	}
}

func TestInstallUploadFailureStopsBeforeRun(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)

	guestRunCalls := 0
	backend := &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		uploadFunc: func(ctx context.Context, emit jobs.EmitFn, req UploadRequest) error {
			return errors.New("disk full")
		},
		guestRunFunc: func(ctx context.Context, emit jobs.EmitFn, req RunRequest) error {
			guestRunCalls++
			return nil
		},
	}
	m.ReplaceBackend(context.Background(), backend)

	jobID := m.Install(InstallRequest{
		VMRef:     "vm-1",
		Username:  "tester",
		Password:  "secret",
		LocalPath: "/tmp/agent.deb",
		GuestOS:   "ubuntu-64",
	})
	job := waitForJob(t, jm, jobID)

	if job.Status != jobs.StatusFailed {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusFailed)
	}
	if !strings.Contains(job.Error, "upload: disk full") {
		t.Fatalf("job error = %q, want upload failure", job.Error)
	}
	if guestRunCalls != 0 {
		t.Fatalf("guestRunCalls = %d, want %d when upload fails", guestRunCalls, 0)
	}
}

func TestInstallUnsupportedPackageFailsBeforeBackendCalls(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)

	uploadCalls := 0
	guestRunCalls := 0
	backend := &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		uploadFunc: func(ctx context.Context, emit jobs.EmitFn, req UploadRequest) error {
			uploadCalls++
			return nil
		},
		guestRunFunc: func(ctx context.Context, emit jobs.EmitFn, req RunRequest) error {
			guestRunCalls++
			return nil
		},
	}
	m.ReplaceBackend(context.Background(), backend)

	jobID := m.Install(InstallRequest{
		VMRef:     "vm-1",
		Username:  "tester",
		Password:  "secret",
		LocalPath: "/tmp/archive.zip",
		GuestOS:   "ubuntu-64",
	})
	job := waitForJob(t, jm, jobID)

	if job.Status != jobs.StatusFailed {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusFailed)
	}
	if !strings.Contains(job.Error, "unsupported package type") {
		t.Fatalf("job error = %q, want unsupported package error", job.Error)
	}
	if uploadCalls != 0 || guestRunCalls != 0 {
		t.Fatalf("backend calls = upload:%d guestRun:%d, want 0 before autodetect failure", uploadCalls, guestRunCalls)
	}
}

func TestInstallCancellationStillRunsBestEffortCleanup(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)

	var guestRunCommands []string
	cleanupCalled := make(chan struct{}, 1)
	backend := &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		uploadFunc: func(ctx context.Context, emit jobs.EmitFn, req UploadRequest) error {
			return nil
		},
		guestRunFunc: func(ctx context.Context, emit jobs.EmitFn, req RunRequest) error {
			guestRunCommands = append(guestRunCommands, req.Command)
			if strings.HasPrefix(req.Command, "rm -f ") {
				if ctx.Err() != nil {
					t.Fatalf("cleanup ran with cancelled context: %v", ctx.Err())
				}
				cleanupCalled <- struct{}{}
				return nil
			}
			<-ctx.Done()
			return ctx.Err()
		},
	}
	m.ReplaceBackend(context.Background(), backend)

	jobID := m.Install(InstallRequest{
		VMRef:     "vm-1",
		Username:  "tester",
		Password:  "secret",
		LocalPath: "/tmp/agent.deb",
		GuestOS:   "ubuntu-64",
	})

	time.Sleep(50 * time.Millisecond)
	jm.Cancel(jobID)
	job := waitForJob(t, jm, jobID)
	if job.Status != jobs.StatusCancelled {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusCancelled)
	}

	select {
	case <-cleanupCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("cleanup was not called after cancellation; guestRunCommands=%v", guestRunCommands)
	}
}

func waitForJob(t *testing.T, jm *jobs.Manager, id string) *jobs.Job {
	t.Helper()

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := jm.Get(id)
		if ok && job.Status != jobs.StatusRunning {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}

	job, _ := jm.Get(id)
	t.Fatalf("job %s did not finish before timeout; last state: %+v", id, job)
	return nil
}
