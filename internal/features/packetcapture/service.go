package packetcapture

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/vim25/types"
	"manosphere/internal/jobs"
	"manosphere/internal/vcenter"
)

// CaptureOptions configures a packet capture session inside a VM guest.
type CaptureOptions struct {
	VMRef     string // ManagedObjectReference value (e.g. "vm-42")
	Username  string // Guest OS username
	Password  string // Guest OS password
	Interface string // Network interface (e.g. "eth0", blank = "any")
	Duration  int    // Capture duration in seconds
	LocalPath string // Where to save the .pcap on the local machine
}

// Service runs packet captures inside VM guests via the Guest Operations API.
// It uses tcpdump (Linux guests) to perform the capture, then retrieves the
// resulting .pcap file via the Guest File Manager.
type Service struct {
	session *vcenter.Session
}

// NewService creates a packetcapture Service.
func NewService(s *vcenter.Session) *Service {
	return &Service{session: s}
}

// Capture runs tcpdump inside the guest, waits for it to finish, then
// downloads the resulting .pcap to localPath on the host machine.
func (s *Service) Capture(ctx context.Context, emit jobs.EmitFn, opts CaptureOptions) error {
	client, err := s.session.Client()
	if err != nil {
		return err
	}

	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: opts.VMRef}
	ops := guest.NewOperationsManager(client.Client, ref)

	auth := &types.NamePasswordAuthentication{
		Username: opts.Username,
		Password: opts.Password,
	}

	pm, err := ops.ProcessManager(ctx)
	if err != nil {
		return fmt.Errorf("getting guest process manager: %w", err)
	}

	fm, err := ops.FileManager(ctx)
	if err != nil {
		return fmt.Errorf("getting guest file manager: %w", err)
	}

	// Temporary capture file path inside the guest
	guestCapturePath := fmt.Sprintf("/tmp/capture_%d.pcap", time.Now().UnixNano())

	iface := opts.Interface
	if iface == "" {
		iface = "any"
	}

	// -G rotates after N seconds, -W 1 limits to one file — together they stop after Duration seconds
	tcpdumpArgs := fmt.Sprintf("-i %s -w %s -G %d -W 1", iface, guestCapturePath, opts.Duration)

	emit(5, "Starting tcpdump in guest...")

	spec := types.GuestProgramSpec{
		ProgramPath:      "/usr/sbin/tcpdump",
		Arguments:        tcpdumpArgs,
		WorkingDirectory: "/tmp",
	}

	pid, err := pm.StartProgram(ctx, auth, &spec)
	if err != nil {
		return fmt.Errorf("starting tcpdump: %w", err)
	}

	emit(10, fmt.Sprintf("tcpdump running (PID %d) for %ds...", pid, opts.Duration))

	startTime := time.Now()
	deadline := startTime.Add(time.Duration(opts.Duration+10) * time.Second)

	for {
		select {
		case <-ctx.Done():
			_ = pm.TerminateProcess(ctx, auth, pid)
			return ctx.Err()
		default:
		}

		procs, err := pm.ListProcesses(ctx, auth, []int64{pid})
		if err != nil {
			return fmt.Errorf("checking tcpdump status: %w", err)
		}

		if len(procs) > 0 && procs[0].ExitCode != -1 {
			if procs[0].ExitCode != 0 {
				return fmt.Errorf("tcpdump exited with code %d", procs[0].ExitCode)
			}
			break
		}

		if time.Now().After(deadline) {
			_ = pm.TerminateProcess(ctx, auth, pid)
			return fmt.Errorf("timed out waiting for tcpdump to finish")
		}

		elapsed := time.Since(startTime)
		pct := int(elapsed.Seconds() / float64(opts.Duration) * 80)
		if pct > 80 {
			pct = 80
		}
		emit(10+pct, fmt.Sprintf("Capturing... (%ds elapsed)", int(elapsed.Seconds())))

		time.Sleep(2 * time.Second)
	}

	emit(90, "Capture complete. Downloading .pcap...")

	fileInfo, err := fm.InitiateFileTransferFromGuest(ctx, auth, guestCapturePath)
	if err != nil {
		return fmt.Errorf("initiating pcap download: %w", err)
	}

	transferURL, err := url.Parse(fileInfo.Url)
	if err != nil {
		return fmt.Errorf("parsing transfer URL: %w", err)
	}

	if err := client.Client.DownloadFile(ctx, opts.LocalPath, transferURL, nil); err != nil {
		return fmt.Errorf("downloading pcap: %w", err)
	}

	// Clean up the temporary file in the guest
	_ = fm.DeleteFile(ctx, auth, guestCapturePath)

	emit(100, fmt.Sprintf("Saved to %s", filepath.Base(opts.LocalPath)))
	return nil
}
