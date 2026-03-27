package manager

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"xman/internal/jobs"
	"xman/internal/sshtransport"
)

// InstallRequest is the payload for installing a package via VMware guest ops.
type InstallRequest struct {
	VMRef     string `json:"vmRef"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	LocalPath string `json:"localPath"`
	GuestOS   string `json:"guestOS"`
	Command   string `json:"command"` // empty = auto-detect from file extension
}

// SSHInstallRequest is the payload for installing a package via SSH/SFTP.
type SSHInstallRequest struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	LocalPath string `json:"localPath"`
	GuestOS   string `json:"guestOS"`
	Command   string `json:"command"` // empty = auto-detect from file extension
}

// isWin reports whether guestOS identifies a Windows guest.
// Handles both VMX short IDs ("win10-64") and display names ("Microsoft Windows 10 (64-bit)").
func isWin(guestOS string) bool {
	lower := strings.ToLower(guestOS)
	return strings.HasPrefix(lower, "win") || strings.Contains(lower, "windows")
}

// guestTempPath returns the path to upload the installer to on the guest.
func guestTempPath(localPath, guestOS string) string {
	filename := filepath.Base(localPath)
	if isWin(guestOS) {
		return `C:\Windows\Temp\` + filename
	}
	return "/tmp/" + filename
}

// autoInstallCommand derives the silent install command from the file extension.
// Returns an error if the extension is unsupported or mismatched with the guest OS.
func autoInstallCommand(localPath, guestPath, guestOS string) (string, error) {
	ext := strings.ToLower(filepath.Ext(localPath))
	win := isWin(guestOS)
	switch ext {
	case ".msi":
		if !win {
			return "", fmt.Errorf(".msi packages require a Windows guest")
		}
		return fmt.Sprintf(`msiexec /i "%s" /qn /norestart`, guestPath), nil
	case ".msix", ".msixbundle":
		if !win {
			return "", fmt.Errorf(".msix packages require a Windows guest")
		}
		// Add-AppxPackage is a PowerShell cmdlet; prefix with powershell -Command so
		// it works through the cmd /c wrapper the VMware backends use for output capture.
		return fmt.Sprintf(`powershell -Command "Add-AppxPackage -Path '%s'"`, guestPath), nil
	case ".exe":
		if !win {
			return "", fmt.Errorf(".exe packages require a Windows guest")
		}
		return fmt.Sprintf(`"%s" /S /SILENT /VERYSILENT /quiet /norestart`, guestPath), nil
	case ".deb":
		if win {
			return "", fmt.Errorf(".deb packages require a Linux guest")
		}
		return fmt.Sprintf(`DEBIAN_FRONTEND=noninteractive dpkg -i "%s" && apt-get install -f -y`, guestPath), nil
	case ".rpm":
		if win {
			return "", fmt.Errorf(".rpm packages require a Linux guest")
		}
		return fmt.Sprintf(`rpm -i "%s"`, guestPath), nil
	default:
		return "", fmt.Errorf("unsupported package type %q (supported: .msi, .exe, .deb, .rpm)", ext)
	}
}

// cleanupCommand returns a best-effort command to delete the installer from the guest.
func cleanupCommand(guestPath, guestOS string) string {
	if isWin(guestOS) {
		return fmt.Sprintf(`cmd /c del /f /q "%s"`, guestPath)
	}
	return fmt.Sprintf(`rm -f "%s"`, guestPath)
}

// Install uploads an installer to the guest via VMware Tools and runs it silently.
func (m *Manager) Install(req InstallRequest) string {
	filename := filepath.Base(req.LocalPath)
	return m.jobs.Submit("install", "Install: "+filename, func(ctx context.Context, emit jobs.EmitFn) error {
		b, err := m.getBackend()
		if err != nil {
			return err
		}

		guestPath := guestTempPath(req.LocalPath, req.GuestOS)

		cmd := req.Command
		if cmd == "" {
			cmd, err = autoInstallCommand(req.LocalPath, guestPath, req.GuestOS)
			if err != nil {
				return err
			}
		}

		emit(5, "Uploading installer...")
		if err := b.Upload(ctx, emit, UploadRequest{
			VMRef:     req.VMRef,
			Username:  req.Username,
			Password:  req.Password,
			LocalPath: req.LocalPath,
			GuestPath: guestPath,
			GuestOS:   req.GuestOS,
		}); err != nil {
			return fmt.Errorf("upload: %w", err)
		}

		emit(55, "Running installer...")
		runErr := b.GuestRun(ctx, emit, RunRequest{
			VMRef:    req.VMRef,
			Username: req.Username,
			Password: req.Password,
			Command:  cmd,
			GuestOS:  req.GuestOS,
		})

		// Best-effort cleanup — ignore errors, suppress output.
		noop := func(_ int, _ string) {}
		_ = b.GuestRun(ctx, noop, RunRequest{
			VMRef:    req.VMRef,
			Username: req.Username,
			Password: req.Password,
			Command:  cleanupCommand(guestPath, req.GuestOS),
			GuestOS:  req.GuestOS,
		})

		return runErr
	})
}

// SSHInstall uploads an installer to the guest via SFTP and runs it silently over SSH.
func (m *Manager) SSHInstall(req SSHInstallRequest) string {
	filename := filepath.Base(req.LocalPath)
	return m.jobs.Submit("install", "SSH Install: "+filename, func(ctx context.Context, emit jobs.EmitFn) error {
		guestPath := guestTempPath(req.LocalPath, req.GuestOS)

		cmd := req.Command
		if cmd == "" {
			var err error
			cmd, err = autoInstallCommand(req.LocalPath, guestPath, req.GuestOS)
			if err != nil {
				return err
			}
		}

		emit(5, "Uploading installer...")
		if err := sshtransport.Upload(ctx, emit, req.Host, req.Port, req.Username, req.Password, req.LocalPath, guestPath); err != nil {
			return fmt.Errorf("upload: %w", err)
		}

		emit(55, "Running installer...")
		runErr := sshtransport.Run(ctx, emit, req.Host, req.Port, req.Username, req.Password, cmd)

		// Best-effort cleanup — ignore errors, suppress output.
		noop := func(_ int, _ string) {}
		_ = sshtransport.Run(ctx, noop, req.Host, req.Port, req.Username, req.Password, cleanupCommand(guestPath, req.GuestOS))

		return runErr
	})
}
