package sshtransport

import (
	"context"
	"fmt"
	"strings"

	"xman/internal/jobs"
)

// DeployKey uploads the public key to the guest VM over SFTP, then runs a
// shell script to append it idempotently to the user's authorized_keys file.
func DeployKey(ctx context.Context, emit jobs.EmitFn, host string, port int, username, password, pubKeyContent string, isWindows bool) error {
	client, display, err := dialWithPassword(host, port, username, password)
	if err != nil {
		return fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()
	emit(5, fmt.Sprintf("Connecting to %s...", display))

	done := make(chan struct{})
	defer close(done)
	go cancelOnContext(ctx, client, done)

	sc, err := newSFTPClient(client)
	if err != nil {
		return fmt.Errorf("SFTP client: %w", err)
	}
	defer sc.Close()

	emit(30, "Uploading public key...")

	var tmpPath string
	if isWindows {
		tmpPath = `C:\Windows\Temp\xman_deploy.pub`
	} else {
		tmpPath = "/tmp/xman_deploy.pub"
	}

	dst, err := sc.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file on guest: %w", err)
	}
	if _, err := dst.Write([]byte(pubKeyContent + "\n")); err != nil {
		dst.Close()
		return fmt.Errorf("writing key to guest: %w", err)
	}
	dst.Close()

	emit(60, "Installing key in authorized_keys...")

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("SSH session: %w", err)
	}
	defer session.Close()

	var cmd string
	if isWindows {
		// Windows OpenSSH uses ProgramData\ssh\administrators_authorized_keys for
		// admin users instead of %USERPROFILE%\.ssh\authorized_keys.
		cmd = `powershell -NoProfile -Command "` +
			`$k=(Get-Content -Raw 'C:\Windows\Temp\xman_deploy.pub').Trim(); ` +
			`$isAdmin=([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator); ` +
			`if($isAdmin){` +
			`  $p=Join-Path $env:ProgramData 'ssh\administrators_authorized_keys'; ` +
			`  if(!(Test-Path $p)){New-Item -ItemType File -Path $p -Force | Out-Null}; ` +
			`} else {` +
			`  $d=Join-Path $env:USERPROFILE '.ssh'; ` +
			`  if(!(Test-Path $d)){New-Item -ItemType Directory -Path $d -Force | Out-Null}; ` +
			`  $p=Join-Path $d 'authorized_keys'; ` +
			`  if(!(Test-Path $p)){New-Item -ItemType File -Path $p -Force | Out-Null}; ` +
			`}; ` +
			`if(!(Select-String -SimpleMatch -Pattern $k -Path $p -Quiet 2>$null)){Add-Content -Path $p -Value $k}; ` +
			`if($isAdmin){icacls.exe $p /inheritance:r /grant 'Administrators:F' /grant 'SYSTEM:F' | Out-Null}; ` +
			`Remove-Item 'C:\Windows\Temp\xman_deploy.pub' -ErrorAction SilentlyContinue"`
	} else {
		// Append key to ~/.ssh/authorized_keys, idempotently.
		cmd = `mkdir -p ~/.ssh && chmod 700 ~/.ssh && ` +
			`grep -qF "$(cat /tmp/xman_deploy.pub)" ~/.ssh/authorized_keys 2>/dev/null || ` +
			`cat /tmp/xman_deploy.pub >> ~/.ssh/authorized_keys; ` +
			`chmod 600 ~/.ssh/authorized_keys; ` +
			`rm -f /tmp/xman_deploy.pub`
	}

	raw, err := session.CombinedOutput(cmd)
	output := strings.TrimSpace(string(raw))
	if err != nil {
		if output != "" {
			return fmt.Errorf("key installation failed: %s (%w)", output, err)
		}
		return fmt.Errorf("key installation failed: %w", err)
	}

	emit(100, "SSH key deployed successfully.")
	return nil
}
