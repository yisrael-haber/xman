package guestexec

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/vim25/types"
	"manosphere/internal/jobs"
	"manosphere/internal/vcenter"
)

// RunOptions configures a guest command execution.
type RunOptions struct {
	VMRef    string
	Username string
	Password string
	Command  string // full command line, e.g. "/usr/bin/df -h"
}

// Service runs commands inside VM guests via the Guest Operations API.
type Service struct {
	session *vcenter.Session
}

// NewService creates a guestexec Service.
func NewService(s *vcenter.Session) *Service {
	return &Service{session: s}
}

const maxOutputBytes = 16 * 1024 // 16 KB cap on returned output

// Run executes a shell command inside the guest, captures combined stdout+stderr,
// and stores the output in the job message on completion.
func (s *Service) Run(ctx context.Context, emit jobs.EmitFn, opts RunOptions) error {
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

	outPath := fmt.Sprintf("/tmp/exec_out_%d.txt", time.Now().UnixNano())

	spec := types.GuestProgramSpec{
		ProgramPath:      "/bin/sh",
		Arguments:        fmt.Sprintf("-c '%s' > %s 2>&1", opts.Command, outPath),
		WorkingDirectory: "/tmp",
	}

	emit(10, "Executing command...")

	pid, err := pm.StartProgram(ctx, auth, &spec)
	if err != nil {
		return fmt.Errorf("starting command: %w", err)
	}

	deadline := time.Now().Add(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			_ = pm.TerminateProcess(ctx, auth, pid)
			return ctx.Err()
		default:
		}

		procs, err := pm.ListProcesses(ctx, auth, []int64{pid})
		if err != nil {
			return fmt.Errorf("checking process status: %w", err)
		}
		if len(procs) > 0 && procs[0].ExitCode != -1 {
			if procs[0].ExitCode != 0 {
				// still download output even on non-zero exit
				break
			}
			break
		}
		if time.Now().After(deadline) {
			_ = pm.TerminateProcess(ctx, auth, pid)
			return fmt.Errorf("timed out waiting for command to finish")
		}
		emit(50, "Waiting for command to finish...")
		time.Sleep(1 * time.Second)
	}

	emit(80, "Downloading output...")

	fileInfo, err := fm.InitiateFileTransferFromGuest(ctx, auth, outPath)
	if err != nil {
		return fmt.Errorf("initiating output download: %w", err)
	}

	transferURL, err := url.Parse(fileInfo.Url)
	if err != nil {
		return fmt.Errorf("parsing transfer URL: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "exec_out_*.txt")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := client.Client.DownloadFile(ctx, tmpPath, transferURL, nil); err != nil {
		return fmt.Errorf("downloading output: %w", err)
	}

	_ = fm.DeleteFile(ctx, auth, outPath)

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("reading output: %w", err)
	}

	output := string(data)
	if len(output) > maxOutputBytes {
		output = output[:maxOutputBytes] + "\n[output truncated]"
	}
	if output == "" {
		output = "(no output)"
	}

	emit(100, output)
	return nil
}
