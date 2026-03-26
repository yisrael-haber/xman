package manager

import (
	"context"

	"xman/internal/jobs"
)

// UploadRequest is the payload sent from the frontend to start an upload.
type UploadRequest struct {
	VMRef     string `json:"vmRef"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	LocalPath string `json:"localPath"`
	GuestPath string `json:"guestPath"`
}

// DownloadRequest is the payload sent from the frontend to start a download.
type DownloadRequest struct {
	VMRef     string `json:"vmRef"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	GuestPath string `json:"guestPath"`
	LocalPath string `json:"localPath"`
}

func (m *Manager) Upload(req UploadRequest) string {
	label := "Upload: " + req.LocalPath + " → " + req.GuestPath
	return m.jobs.Submit("filetransfer", label, func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		return b.Upload(ctx, emit, req)
	})
}

func (m *Manager) Download(req DownloadRequest) string {
	label := "Download: " + req.GuestPath + " → " + req.LocalPath
	return m.jobs.Submit("filetransfer", label, func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		return b.Download(ctx, emit, req)
	})
}
