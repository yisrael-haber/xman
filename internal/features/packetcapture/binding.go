package packetcapture

import (
	"context"

	"manosphere/internal/jobs"
	"manosphere/internal/vcenter"
)

// CaptureRequest is the payload sent from the frontend to start a capture.
type CaptureRequest struct {
	VMRef     string `json:"vmRef"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Interface string `json:"interface"` // blank = "any"
	Duration  int    `json:"duration"`  // seconds
	LocalPath string `json:"localPath"` // where to save the .pcap
}

// Binding exposes packet capture operations to the Wails frontend.
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

// StartCapture begins a packet capture inside the VM guest and returns a job ID immediately.
// Progress can be tracked by listening to the "job:<id>" event on the frontend.
func (b *Binding) StartCapture(req CaptureRequest) string {
	label := "Capture on " + req.VMRef
	return b.jobs.Submit("packetcapture", label, func(ctx context.Context, emit jobs.EmitFn) error {
		return b.service.Capture(ctx, emit, CaptureOptions{
			VMRef:     req.VMRef,
			Username:  req.Username,
			Password:  req.Password,
			Interface: req.Interface,
			Duration:  req.Duration,
			LocalPath: req.LocalPath,
		})
	})
}
