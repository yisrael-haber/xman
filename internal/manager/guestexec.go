package manager

import (
	"context"

	"xman/internal/jobs"
)

// RunRequest is the payload sent from the frontend to run a guest command.
type RunRequest struct {
	VMRef           string `json:"vmRef"`
	CredentialLabel string `json:"credentialLabel"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	Command         string `json:"command"`
	GuestOS         string `json:"guestOS"` // used by backends to select the right shell
}

func (m *Manager) guestRun(req RunRequest) string {
	return m.submitJob("guestexec", summarizeCommandLabel("$ ", req.Command), func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getGuestOpsBackend()
		if err != nil {
			return err
		}
		req, err = resolveRunRequestCredential(req)
		if err != nil {
			return err
		}
		return b.GuestRun(ctx, emit, req)
	})
}
