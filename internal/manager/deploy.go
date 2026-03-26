package manager

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"xman/internal/jobs"
)

// DeployRequest is the payload to upload an installer and execute it in the guest.
type DeployRequest struct {
	VMRef      string `json:"vmRef"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	LocalPath  string `json:"localPath"`
	RunCommand string `json:"runCommand"` // optional; auto-derived from file extension if empty
}

// DerivedRunCommand returns a default silent-install command for the given guest path.
func DerivedRunCommand(isWindows bool, guestPath string) string {
	ext := strings.ToLower(filepath.Ext(guestPath))
	if isWindows {
		switch ext {
		case ".msi":
			return fmt.Sprintf(`msiexec /i "%s" /qn /norestart`, guestPath)
		case ".exe":
			return fmt.Sprintf(`"%s" /S`, guestPath)
		default:
			return fmt.Sprintf(`"%s"`, guestPath)
		}
	}
	switch ext {
	case ".sh":
		return fmt.Sprintf(`bash "%s"`, guestPath)
	case ".deb":
		return fmt.Sprintf(`dpkg -i "%s"`, guestPath)
	case ".rpm":
		return fmt.Sprintf(`rpm -i "%s"`, guestPath)
	default:
		return guestPath
	}
}

func (m *Manager) VMDeployAndRun(req DeployRequest) string {
	filename := filepath.Base(req.LocalPath)
	return m.jobs.Submit("deploy", "Deploy: "+filename, func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}
		return b.DeployAndRun(ctx, emit, req)
	})
}
