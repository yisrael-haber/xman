package manager

import (
	"context"
	"fmt"
	"strings"

	"xman/internal/config"
	"xman/internal/jobs"
	"xman/internal/sshtransport"
)

// DeploySSHKeyRequest is the payload for deploying a stored key to a guest VM.
type DeploySSHKeyRequest struct {
	Label    string `json:"label"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	GuestOS  string `json:"guestOS"`
}

// DeploySSHKey uploads the named key's public half to the guest VM and
// appends it to authorized_keys for the connecting user.
func (m *Manager) DeploySSHKey(req DeploySSHKeyRequest) string {
	return m.submitJob("deploy-key", "Deploy Key: "+req.Label, func(ctx context.Context, emit jobs.EmitFn) error {
		meta, err := config.GetKey(req.Label)
		if err != nil {
			return fmt.Errorf("key %q not found: %w", req.Label, err)
		}
		if err := sshtransport.DeployKey(ctx, emit,
			req.Host, req.Port, req.Username, req.Password,
			meta.PublicKey, IsWindows(req.GuestOS),
		); err != nil {
			return err
		}

		deployUser := strings.TrimSpace(req.Username)
		if deployUser != strings.TrimSpace(meta.DefaultUser) {
			if _, err := config.UpdateKeyDefaultUser(req.Label, deployUser); err != nil {
				return fmt.Errorf("key deployed, but failed to update default user for %q: %w", req.Label, err)
			}
		}

		emit(85, "Verifying key-based SSH login...")
		if err := sshtransport.VerifyKey(ctx, req.Host, req.Label); err != nil {
			return fmt.Errorf("key was copied, but verification failed: %w", err)
		}
		return nil
	})
}
