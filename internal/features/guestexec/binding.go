package guestexec

import (
	"context"

	"manosphere/internal/jobs"
	"manosphere/internal/vcenter"
)

// RunRequest is the payload sent from the frontend to run a guest command.
type RunRequest struct {
	VMRef    string `json:"vmRef"`
	Username string `json:"username"`
	Password string `json:"password"`
	Command  string `json:"command"`
}

// Binding exposes guest command execution to the Wails frontend.
type Binding struct {
	service *Service
	jobs    *jobs.Manager
	ctx     context.Context
}

// NewBinding creates a Binding backed by the given vCenter session and job manager.
func NewBinding(s *vcenter.Session, jm *jobs.Manager) *Binding {
	return &Binding{service: NewService(s), jobs: jm}
}

// SetContext is called by app.go on startup to provide the Wails runtime context.
func (b *Binding) SetContext(ctx context.Context) {
	b.ctx = ctx
}

// GuestRun executes a command inside the VM guest and returns a job ID immediately.
// When the job completes, the job's Message field contains the command output.
func (b *Binding) GuestRun(req RunRequest) string {
	label := "$ " + req.Command
	return b.jobs.Submit("guestexec", label, func(ctx context.Context, emit jobs.EmitFn) error {
		return b.service.Run(ctx, emit, RunOptions{
			VMRef:    req.VMRef,
			Username: req.Username,
			Password: req.Password,
			Command:  req.Command,
		})
	})
}
