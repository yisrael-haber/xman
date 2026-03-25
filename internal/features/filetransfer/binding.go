package filetransfer

import (
	"context"

	"manosphere/internal/jobs"
	"manosphere/internal/vcenter"
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

// Binding exposes file transfer operations to the Wails frontend.
type Binding struct {
	service *Service
	jobs    *jobs.Manager
	ctx     context.Context
}

// NewBinding creates a Binding backed by the given vCenter session and job manager.
func NewBinding(s *vcenter.Session, jm *jobs.Manager) *Binding {
	return &Binding{
		service: NewService(s),
		jobs:    jm,
	}
}

// SetContext is called by app.go on startup to provide the Wails runtime context.
func (b *Binding) SetContext(ctx context.Context) {
	b.ctx = ctx
}

// Upload starts a file upload job and returns the job ID immediately.
// Progress can be tracked by listening to the "job:<id>" event on the frontend.
func (b *Binding) Upload(req UploadRequest) string {
	label := "Upload: " + req.LocalPath + " → " + req.GuestPath
	return b.jobs.Submit("filetransfer", label, func(ctx context.Context, emit jobs.EmitFn) error {
		return b.service.Upload(ctx, emit, req.VMRef, GuestAuth{
			Username: req.Username,
			Password: req.Password,
		}, req.LocalPath, req.GuestPath)
	})
}

// Download starts a file download job and returns the job ID immediately.
func (b *Binding) Download(req DownloadRequest) string {
	label := "Download: " + req.GuestPath + " → " + req.LocalPath
	return b.jobs.Submit("filetransfer", label, func(ctx context.Context, emit jobs.EmitFn) error {
		return b.service.Download(ctx, emit, req.VMRef, GuestAuth{
			Username: req.Username,
			Password: req.Password,
		}, req.GuestPath, req.LocalPath)
	})
}
