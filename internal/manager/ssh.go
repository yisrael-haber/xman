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
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Command  string `json:"command"`
}

// SSHTransferRequest is the payload for SFTP upload/download.
type SSHTransferRequest struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	LocalPath string `json:"localPath"`
	GuestPath string `json:"guestPath"`
}

func (m *Manager) SSHRun(req SSHRunRequest) string {
	return m.jobs.Submit("exec", "SSH: "+req.Command, func(ctx context.Context, emit jobs.EmitFn) error {
		return sshtransport.Run(ctx, emit, req.Host, req.Port, req.Username, req.Password, req.Command)
	})
}

func (m *Manager) SSHUpload(req SSHTransferRequest) string {
	return m.jobs.Submit("upload", "SSH Upload: "+filepath.Base(req.LocalPath), func(ctx context.Context, emit jobs.EmitFn) error {
		return sshtransport.Upload(ctx, emit, req.Host, req.Port, req.Username, req.Password, req.LocalPath, req.GuestPath)
	})
}

func (m *Manager) SSHDownload(req SSHTransferRequest) string {
	return m.jobs.Submit("download", "SSH Download: "+req.GuestPath, func(ctx context.Context, emit jobs.EmitFn) error {
		return sshtransport.Download(ctx, emit, req.Host, req.Port, req.Username, req.Password, req.GuestPath, req.LocalPath)
	})
}
