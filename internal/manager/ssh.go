package manager

import (
	"context"
	"path/filepath"

	"xman/internal/jobs"
	"xman/internal/sshtransport"
)

// SSHRunRequest is the payload for running a command over SSH.
type SSHRunRequest struct {
	Host     string `json:"host"`
	KeyLabel string `json:"keyLabel"`
	Command  string `json:"command"`
}

// SSHTransferRequest is the payload for SFTP upload/download.
type SSHTransferRequest struct {
	Host      string `json:"host"`
	KeyLabel  string `json:"keyLabel"`
	LocalPath string `json:"localPath"`
	GuestPath string `json:"guestPath"`
}

func (m *Manager) sshRun(req SSHRunRequest) string {
	return m.submitJob("exec", summarizeCommandLabel("SSH: ", req.Command), func(ctx context.Context, emit jobs.EmitFn) error {
		return sshtransport.Run(ctx, emit, req.Host, req.KeyLabel, req.Command)
	})
}

func (m *Manager) sshUpload(req SSHTransferRequest) string {
	return m.submitJob("upload", "SSH Upload: "+filepath.Base(req.LocalPath), func(ctx context.Context, emit jobs.EmitFn) error {
		return sshtransport.Upload(ctx, emit, req.Host, req.KeyLabel, req.LocalPath, req.GuestPath)
	})
}

func (m *Manager) sshDownload(req SSHTransferRequest) string {
	return m.submitJob("download", "SSH Download: "+req.GuestPath, func(ctx context.Context, emit jobs.EmitFn) error {
		return sshtransport.Download(ctx, emit, req.Host, req.KeyLabel, req.GuestPath, req.LocalPath)
	})
}
