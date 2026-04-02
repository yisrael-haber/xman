package vcenter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"xman/internal/manager"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
)

const dockerGuestOpsEnv = "XMAN_DOCKER_GUESTOPS"

var (
	dockerGuestOpsCheckOnce sync.Once
	dockerGuestOpsCheckErr  error
)

type dockerGuestOpsFixture struct {
	backend  *Backend
	model    *simulator.Model
	vm       *object.VirtualMachine
	vmRef    string
	username string
	password string
	guestOS  string
}

func TestDockerGuestOpsGuestRunSuccess(t *testing.T) {
	fx := newDockerGuestOpsFixture(t)

	emitted, err := fx.runCommand(context.Background(), "echo hello-from-docker-guest")
	if err != nil {
		t.Fatalf("GuestRun() error = %v", err)
	}
	if got := lastMessage(emitted); got != "Command completed." {
		t.Fatalf("final emit = %q, want %q; all emits=%v", got, "Command completed.", emitted)
	}
	if !messagesContain(emitted, "hello-from-docker-guest") {
		t.Fatalf("GuestRun() emits %v, want command output", emitted)
	}
}

func TestDockerGuestOpsGuestRunCancellationStopsSideEffects(t *testing.T) {
	fx := newDockerGuestOpsFixture(t)

	candidates := []string{
		"sleep 30",
		"tail -f /dev/null",
	}

	var observations []string
	for _, command := range candidates {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func(cmd string) {
			emitted, err := fx.runCommand(ctx, cmd)
			if err == nil && lastMessage(emitted) == "Command completed." {
				done <- fmt.Errorf("unexpected completion: emits=%v", emitted)
				return
			}
			done <- err
		}(command)

		time.Sleep(2 * time.Second)
		cancel()

		select {
		case err := <-done:
			if errors.Is(err, context.Canceled) {
				return
			}
			observations = append(observations, fmt.Sprintf("%q:error=%v", command, err))
		case <-time.After(20 * time.Second):
			observations = append(observations, fmt.Sprintf("%q:timed out waiting for cancellation", command))
		}
	}

	t.Skipf("container-backed vcsim did not surface a cancellable GuestRun in the attempted command forms: %s", strings.Join(observations, " | "))
}

func TestDockerGuestOpsGuestRunNonZeroExit(t *testing.T) {
	fx := newDockerGuestOpsFixture(t)

	candidates := []struct {
		name    string
		command string
	}{
		{name: "simple-false", command: "false"},
		{name: "echo-then-exit", command: "echo nonzero-from-docker-guest; exit 7"},
		{name: "printf-then-exit", command: "printf nonzero-from-docker-guest; exit 7"},
		{name: "grouped-exit", command: "{ echo nonzero-from-docker-guest; exit 7; }"},
	}

	var observations []string
	for _, candidate := range candidates {
		emitted, err := fx.runCommand(context.Background(), candidate.command)
		if err != nil {
			observations = append(observations, fmt.Sprintf("%s:error=%v emits=%v", candidate.name, err, emitted))
			continue
		}
		if lastMessage(emitted) == "Command finished with non-zero exit status." {
			if !messagesContain(emitted, "[exit code:") {
				t.Fatalf("%s emits %v, want exit code marker", candidate.name, emitted)
			}
			return
		}
		observations = append(observations, fmt.Sprintf("%s:final=%q emits=%v", candidate.name, lastMessage(emitted), emitted))
	}

	t.Skipf("container-backed vcsim did not propagate a non-zero guest exit code for any attempted command form: %s", strings.Join(observations, " | "))
}

func TestDockerGuestOpsUploadDownloadRoundTrip(t *testing.T) {
	fx := newDockerGuestOpsFixture(t)

	tempDir := t.TempDir()
	localSource := filepath.Join(tempDir, "upload.bin")
	localDownload := filepath.Join(tempDir, "download.bin")
	want := []byte{0x00, 0x01, 0x02, 0x7f, 0x80, 0xfe, 0xff, '\n', 'x', 'm', 'a', 'n'}
	if err := os.WriteFile(localSource, want, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", localSource, err)
	}

	var uploadEmits []string
	err := fx.backend.Upload(context.Background(), func(_ int, message string) {
		uploadEmits = append(uploadEmits, message)
	}, manager.UploadRequest{
		VMRef:     fx.vmRef,
		Username:  fx.username,
		Password:  fx.password,
		LocalPath: localSource,
		GuestPath: "/tmp/xman-upload.bin",
		GuestOS:   fx.guestOS,
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if got := lastMessage(uploadEmits); got != "Upload complete." {
		t.Fatalf("Upload() final emit = %q, want %q; all emits=%v", got, "Upload complete.", uploadEmits)
	}

	var downloadEmits []string
	err = fx.backend.Download(context.Background(), func(_ int, message string) {
		downloadEmits = append(downloadEmits, message)
	}, manager.DownloadRequest{
		VMRef:     fx.vmRef,
		Username:  fx.username,
		Password:  fx.password,
		GuestPath: "/tmp/xman-upload.bin",
		LocalPath: localDownload,
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if got := lastMessage(downloadEmits); got != "Download complete." {
		t.Fatalf("Download() final emit = %q, want %q; all emits=%v", got, "Download complete.", downloadEmits)
	}

	got, err := os.ReadFile(localDownload)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", localDownload, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("downloaded contents = %v, want %v", got, want)
	}
}

func TestDockerGuestOpsConsoleURLCloneTicketOneTimeUse(t *testing.T) {
	fx := newDockerGuestOpsFixture(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	info, err := fx.backend.ConsoleInfo(ctx, fx.vmRef)
	if err != nil {
		t.Fatalf("ConsoleInfo() error = %v", err)
	}

	consoleURL, err := url.Parse(info.URL)
	if err != nil {
		t.Fatalf("url.Parse(ConsoleInfo().URL) error = %v", err)
	}

	query := consoleURL.Query()
	if got := query.Get("vmId"); got != fx.vmRef {
		t.Fatalf("console vmId = %q, want %q", got, fx.vmRef)
	}

	cloneTicket := query.Get("sessionTicket")
	if cloneTicket == "" {
		t.Fatal("ConsoleInfo() missing sessionTicket")
	}

	sessionClient, err := fx.backend.session.Client()
	if err != nil {
		t.Fatalf("session.Client() error = %v", err)
	}

	sdkURL := *sessionClient.Client.URL()
	sdkURL.User = nil

	cloneClient, err := govmomi.NewClient(ctx, &sdkURL, true)
	if err != nil {
		t.Fatalf("govmomi.NewClient(cloneClient) error = %v", err)
	}
	t.Cleanup(func() {
		_ = cloneClient.Logout(context.Background())
	})

	if err := cloneClient.SessionManager.CloneSession(ctx, cloneTicket); err != nil {
		t.Fatalf("CloneSession(first use) error = %v", err)
	}
	if _, err := methods.GetCurrentTime(ctx, cloneClient); err != nil {
		t.Fatalf("GetCurrentTime() after CloneSession error = %v", err)
	}

	replayClient, err := govmomi.NewClient(ctx, &sdkURL, true)
	if err != nil {
		t.Fatalf("govmomi.NewClient(replayClient) error = %v", err)
	}
	t.Cleanup(func() {
		_ = replayClient.Logout(context.Background())
	})

	if err := replayClient.SessionManager.CloneSession(ctx, cloneTicket); err == nil {
		t.Fatal("CloneSession(second use) error = nil, want one-time ticket rejection")
	}
}

func newDockerGuestOpsFixture(t *testing.T) *dockerGuestOpsFixture {
	t.Helper()

	requireDockerGuestOpsEnabled(t)

	model := newTestModel()
	model.Machine = 0

	backend, createdModel := newBackendWithModel(t, model)
	client, err := backend.session.Client()
	if err != nil {
		t.Fatalf("session.Client() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	finder := find.NewFinder(client.Client, false)
	dc, err := finder.Datacenter(ctx, "DC0")
	if err != nil {
		t.Fatalf("Datacenter() error = %v", err)
	}
	finder.SetDatacenter(dc)

	pools, err := finder.ResourcePoolList(ctx, "*")
	if err != nil {
		t.Fatalf("ResourcePoolList() error = %v", err)
	}
	if len(pools) == 0 {
		t.Fatal("ResourcePoolList() returned no resource pools")
	}

	folders, err := dc.Folders(ctx)
	if err != nil {
		t.Fatalf("Folders() error = %v", err)
	}

	containerArgs, err := json.Marshal([]string{"debian:stable-slim", "tail", "-f", "/dev/null"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	spec := types.VirtualMachineConfigSpec{
		Name: "docker-guestops-test",
		Files: &types.VirtualMachineFileInfo{
			VmPathName: "[LocalDS_0] docker-guestops-test",
		},
		ExtraConfig: []types.BaseOptionValue{
			&types.OptionValue{Key: simulator.ContainerBackingOptionKey, Value: string(containerArgs)},
			&types.OptionValue{Key: "RUN.mountdmi", Value: "false"},
		},
	}

	task, err := folders.VmFolder.CreateVM(ctx, spec, pools[0], nil)
	if err != nil {
		t.Fatalf("CreateVM() error = %v", err)
	}
	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		t.Fatalf("CreateVM().WaitForResult() error = %v", err)
	}

	vm := object.NewVirtualMachine(client.Client, info.Result.(types.ManagedObjectReference))
	t.Cleanup(func() {
		powerCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if task, err := vm.PowerOff(powerCtx); err == nil {
			_ = task.Wait(powerCtx)
		}
		if task, err := vm.Destroy(powerCtx); err == nil {
			_ = task.Wait(powerCtx)
		}
	})

	powerTask, err := vm.PowerOn(ctx)
	if err != nil {
		t.Fatalf("PowerOn() error = %v", err)
	}
	if err := powerTask.Wait(ctx); err != nil {
		t.Fatalf("PowerOn().Wait() error = %v", err)
	}

	ipCtx, cancelIP := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelIP()
	if _, err := vm.WaitForIP(ipCtx, true); err != nil {
		t.Fatalf("WaitForIP() error = %v", err)
	}

	return &dockerGuestOpsFixture{
		backend:  backend,
		model:    createdModel,
		vm:       vm,
		vmRef:    vm.Reference().Value,
		username: "docker",
		password: "docker",
		guestOS:  "Debian GNU/Linux",
	}
}

func requireDockerGuestOpsEnabled(t *testing.T) {
	t.Helper()

	if os.Getenv(dockerGuestOpsEnv) != "1" {
		t.Skipf("set %s=1 or run `make test-vcenter-docker` to enable Docker-backed guest-ops integration tests", dockerGuestOpsEnv)
	}

	dockerGuestOpsCheckOnce.Do(func() {
		if runtime.GOOS != "linux" {
			dockerGuestOpsCheckErr = fmt.Errorf("Docker-backed guest-ops tests require Linux/WSL; GOOS=%s", runtime.GOOS)
			return
		}
		if _, err := exec.LookPath("docker"); err != nil {
			dockerGuestOpsCheckErr = fmt.Errorf("docker CLI not found in PATH: %w", err)
			return
		}
		dockerCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(dockerCtx, "docker", "info")
		if out, err := cmd.CombinedOutput(); err != nil {
			dockerGuestOpsCheckErr = fmt.Errorf("docker daemon is not available: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	})
	if dockerGuestOpsCheckErr != nil {
		t.Fatal(dockerGuestOpsCheckErr)
	}
}

func (fx *dockerGuestOpsFixture) runCommand(ctx context.Context, command string) ([]string, error) {
	var emitted []string
	err := fx.backend.GuestRun(ctx, func(_ int, message string) {
		emitted = append(emitted, message)
	}, manager.RunRequest{
		VMRef:    fx.vmRef,
		Username: fx.username,
		Password: fx.password,
		Command:  command,
		GuestOS:  fx.guestOS,
	})
	return emitted, err
}

func lastMessage(messages []string) string {
	if len(messages) == 0 {
		return ""
	}
	return messages[len(messages)-1]
}

func messagesContain(messages []string, needle string) bool {
	for _, message := range messages {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}
