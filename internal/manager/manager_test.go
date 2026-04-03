package manager

import (
	"context"
	"errors"
	"net"
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

	listVMsFunc         func(context.Context) ([]VMInfo, error)
	getVMFunc           func(context.Context, string) (VMInfo, error)
	powerOnFunc         func(context.Context, string) error
	powerOffFunc        func(context.Context, string) error
	resetFunc           func(context.Context, string) error
	suspendFunc         func(context.Context, string) error
	updateVMConfigFunc  func(context.Context, jobs.EmitFn, VMConfigUpdateRequest) error
	listSnapshotsFunc   func(context.Context, string) ([]SnapshotInfo, error)
	createSnapshotFunc  func(context.Context, jobs.EmitFn, CreateSnapshotRequest) error
	revertSnapshotFunc  func(context.Context, jobs.EmitFn, string) error
	deleteSnapshotFunc  func(context.Context, jobs.EmitFn, string, bool) error
	uploadFunc          func(context.Context, jobs.EmitFn, UploadRequest) error
	downloadFunc        func(context.Context, jobs.EmitFn, DownloadRequest) error
	guestRunFunc        func(context.Context, jobs.EmitFn, RunRequest) error
	deleteGuestFileFunc func(context.Context, string, string, string, string) error
	listHostsFunc       func(context.Context) ([]HostInfo, error)
	listDatastoresFunc  func(context.Context) ([]DatastoreInfo, error)
	listNetworksFunc    func(context.Context) (NetworkSummary, error)
	listVMNetworksFunc  func(context.Context, string) ([]VMNetworkOption, error)
	updateVMNetworkFunc func(context.Context, jobs.EmitFn, VMNetworkUpdateRequest) error
	installToolsFunc    func(context.Context, jobs.EmitFn, string) error
	consoleInfoFunc     func(context.Context, string) (ConsoleLaunchInfo, error)
}

func (b *fakeBackend) BackendType() string { return b.backendType }
func (b *fakeBackend) DisplayName() string { return b.displayName }
func (b *fakeBackend) Capabilities() Capabilities {
	caps := b.capabilities
	if caps != (Capabilities{}) {
		return caps
	}
	return Capabilities{
		GuestOps:     b.uploadFunc != nil || b.downloadFunc != nil || b.guestRunFunc != nil,
		Inventory:    b.listHostsFunc != nil || b.listDatastoresFunc != nil,
		ToolsInstall: b.installToolsFunc != nil,
		Console:      b.consoleInfoFunc != nil,
	}
}
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
func (b *fakeBackend) UpdateVMConfig(ctx context.Context, emit jobs.EmitFn, req VMConfigUpdateRequest) error {
	if b.updateVMConfigFunc != nil {
		return b.updateVMConfigFunc(ctx, emit, req)
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
func (b *fakeBackend) DeleteGuestFile(ctx context.Context, vmRef, username, password, guestPath string) error {
	if b.deleteGuestFileFunc != nil {
		return b.deleteGuestFileFunc(ctx, vmRef, username, password, guestPath)
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
func (b *fakeBackend) ListVMNetworkOptions(ctx context.Context, vmRef string) ([]VMNetworkOption, error) {
	if b.listVMNetworksFunc != nil {
		return b.listVMNetworksFunc(ctx, vmRef)
	}
	return nil, nil
}
func (b *fakeBackend) UpdateVMNetwork(ctx context.Context, emit jobs.EmitFn, req VMNetworkUpdateRequest) error {
	if b.updateVMNetworkFunc != nil {
		return b.updateVMNetworkFunc(ctx, emit, req)
	}
	return nil
}
func (b *fakeBackend) ConsoleInfo(ctx context.Context, vmRef string) (ConsoleLaunchInfo, error) {
	if b.consoleInfoFunc != nil {
		return b.consoleInfoFunc(ctx, vmRef)
	}
	return ConsoleLaunchInfo{}, nil
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
			Console:      false,
		},
	}

	m.ReplaceBackend(context.Background(), backend)

	info := m.ConnectionInfo()
	if info.BackendType != "workstation" || info.DisplayName != "Local Workstation" {
		t.Fatalf("ConnectionInfo() = %+v, want backend metadata", info)
	}
	if !info.GuestOps || info.Inventory || !info.ToolsInstall || info.Console {
		t.Fatalf("ConnectionInfo() capabilities = %+v, want GuestOps=true Inventory=false ToolsInstall=true Console=false", info)
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

func TestVMConsoleURLRejectsUnsupportedBackend(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)
	m.ReplaceBackend(context.Background(), &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		capabilities: Capabilities{
			GuestOps:     true,
			ToolsInstall: true,
		},
	})

	if _, err := m.vmConsoleURL("vm-1"); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("VMConsoleURL() error = %v, want unsupported backend error", err)
	}
}

func TestVMConsoleInfoAddsReachabilityAndRedactedURL(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	jm := jobs.NewManager(nil)
	m := New(jm)
	m.ReplaceBackend(context.Background(), &fakeBackend{
		backendType: "vcenter",
		displayName: "Test VC",
		capabilities: Capabilities{
			Console: true,
		},
		consoleInfoFunc: func(context.Context, string) (ConsoleLaunchInfo, error) {
			return ConsoleLaunchInfo{
				URL:           "https://127.0.0.1/ui/webconsole.html?vmId=vm-1&sessionTicket=abcdef1234567890",
				VCenterURL:    "http://" + listener.Addr().String(),
				ConsoleHost:   "127.0.0.1",
				TicketPreview: "abcdef...7890",
			}, nil
		},
	})

	info, err := m.vmConsoleInfo("vm-1")
	if err != nil {
		t.Fatalf("VMConsoleInfo() error = %v", err)
	}
	if info.DisplayURL == info.URL || !strings.Contains(info.DisplayURL, "sessionTicket=abcdef...7890") {
		t.Fatalf("DisplayURL = %q, want redacted session ticket based on URL %q", info.DisplayURL, info.URL)
	}
	if !info.VCenterCheck.Reachable {
		t.Fatalf("VCenterCheck = %+v, want reachable", info.VCenterCheck)
	}
	if !info.ConsoleHostCheck.Reachable {
		t.Fatalf("ConsoleHostCheck = %+v, want reachable", info.ConsoleHostCheck)
	}
}

func TestVMNetworkOptionsUsesBackendImplementation(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)
	want := []VMNetworkOption{
		{ID: "network-a", Name: "App Network", Type: "Standard"},
	}
	m.ReplaceBackend(context.Background(), &fakeBackend{
		backendType: "vcenter",
		displayName: "Test VC",
		listVMNetworksFunc: func(_ context.Context, vmRef string) ([]VMNetworkOption, error) {
			if vmRef != "vm-42" {
				t.Fatalf("vmRef = %q, want %q", vmRef, "vm-42")
			}
			return want, nil
		},
	})

	got, err := m.vmNetworkOptions("vm-42")
	if err != nil {
		t.Fatalf("VMNetworkOptions() error = %v", err)
	}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("VMNetworkOptions() = %+v, want %+v", got, want)
	}
}

func TestVMUpdateNetworkSubmitsJobToBackend(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)
	req := VMNetworkUpdateRequest{
		VMRef:     "vm-42",
		AdapterID: "nic-1",
		NetworkID: "network-a",
		Connected: true,
	}
	called := make(chan VMNetworkUpdateRequest, 1)
	m.ReplaceBackend(context.Background(), &fakeBackend{
		backendType: "vcenter",
		displayName: "Test VC",
		updateVMNetworkFunc: func(_ context.Context, _ jobs.EmitFn, got VMNetworkUpdateRequest) error {
			called <- got
			return nil
		},
	})

	jobID := m.vmUpdateNetwork(req)
	if jobID == "" {
		t.Fatal("VMUpdateNetwork() returned empty job ID")
	}

	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()

	select {
	case got := <-called:
		if got != req {
			t.Fatalf("UpdateVMNetwork() request = %+v, want %+v", got, req)
		}
	case <-deadline.C:
		t.Fatal("timed out waiting for UpdateVMNetwork backend call")
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
	getCallsAfterPowerOff := 0
	backend := &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		powerOffFunc: func(context.Context, string) error {
			mu.Lock()
			defer mu.Unlock()
			powerOffCalls++
			return nil
		},
		getVMFunc: func(context.Context, string) (VMInfo, error) {
			mu.Lock()
			defer mu.Unlock()
			state := "poweredOn"
			if powerOffCalls > 0 {
				getCallsAfterPowerOff++
				if getCallsAfterPowerOff >= 2 {
					state = "poweredOff"
				}
			}
			return VMInfo{
				Ref:        "vm-1",
				Name:       "vm-1",
				PowerState: state,
			}, nil
		},
	}
	m.ReplaceBackend(context.Background(), backend)

	jobID := m.vmPowerOff("vm-1")
	job := waitForJob(t, jm, jobID)

	if job.Status != jobs.StatusDone {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusDone)
	}
	if powerOffCalls != 1 {
		t.Fatalf("PowerOff call count = %d, want %d", powerOffCalls, 1)
	}
	if getCallsAfterPowerOff < 2 {
		t.Fatalf("GetVM after PowerOff called %d times, want at least %d to prove wait loop ran", getCallsAfterPowerOff, 2)
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

	jobID := m.snapshotCreate(CreateSnapshotRequest{
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

	jobID := m.guestRun(RunRequest{
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

func TestGuestRunResolvesStoredCredentialLabelBeforeDelegating(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)

	previousLoader := loadGuestCredential
	loadGuestCredential = func(label string) (config.GuestCredential, error) {
		if label != "linux-admin" {
			t.Fatalf("loadGuestCredential label = %q, want %q", label, "linux-admin")
		}
		return config.GuestCredential{
			GuestCredentialMeta: config.GuestCredentialMeta{
				Label:    "linux-admin",
				Username: "tester",
			},
			Password: "secret",
		}, nil
	}
	defer func() { loadGuestCredential = previousLoader }()

	var delegatedReq RunRequest
	backend := &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		guestRunFunc: func(ctx context.Context, emit jobs.EmitFn, req RunRequest) error {
			delegatedReq = req
			emit(100, "Command completed.")
			return nil
		},
	}
	m.ReplaceBackend(context.Background(), backend)

	jobID := m.guestRun(RunRequest{
		VMRef:           "vm-1",
		CredentialLabel: "linux-admin",
		Command:         "hostname",
		GuestOS:         "ubuntu-64",
	})
	job := waitForJob(t, jm, jobID)

	if job.Status != jobs.StatusDone {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusDone)
	}
	if delegatedReq.Username != "tester" || delegatedReq.Password != "secret" {
		t.Fatalf("delegated guest run req = %+v, want resolved stored credential", delegatedReq)
	}
}

func TestUploadCredentialLookupFailureFailsJobBeforeDelegating(t *testing.T) {
	jm := jobs.NewManager(nil)
	m := New(jm)

	previousLoader := loadGuestCredential
	loadGuestCredential = func(label string) (config.GuestCredential, error) {
		return config.GuestCredential{}, errors.New("credential not found")
	}
	defer func() { loadGuestCredential = previousLoader }()

	uploadCalls := 0
	backend := &fakeBackend{
		backendType: "workstation",
		displayName: "Local Workstation",
		uploadFunc: func(ctx context.Context, emit jobs.EmitFn, req UploadRequest) error {
			uploadCalls++
			return nil
		},
	}
	m.ReplaceBackend(context.Background(), backend)

	jobID := m.upload(UploadRequest{
		VMRef:           "vm-1",
		CredentialLabel: "missing",
		LocalPath:       "/tmp/local.iso",
		GuestPath:       "/tmp/guest.iso",
		GuestOS:         "ubuntu-64",
	})
	job := waitForJob(t, jm, jobID)

	if job.Status != jobs.StatusFailed {
		t.Fatalf("job status = %q, want %q", job.Status, jobs.StatusFailed)
	}
	if !strings.Contains(job.Error, "credential not found") {
		t.Fatalf("job error = %q, want credential lookup failure", job.Error)
	}
	if uploadCalls != 0 {
		t.Fatalf("uploadCalls = %d, want %d when credential lookup fails", uploadCalls, 0)
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

	uploadID := m.upload(UploadRequest{
		VMRef:     "vm-1",
		Username:  "tester",
		Password:  "secret",
		LocalPath: "/tmp/local.iso",
		GuestPath: "/tmp/guest.iso",
		GuestOS:   "ubuntu-64",
	})
	downloadID := m.download(DownloadRequest{
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
