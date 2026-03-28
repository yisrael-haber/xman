package main

import (
	"fmt"
	"net"
	"os/exec"
	goRuntime "runtime"
	"strconv"
	"strings"

	"xman/internal/config"
)

// LaunchInteractiveSSHSession opens the native OS terminal and starts the
// system SSH client against the selected host using the stored private key.
func (a *App) LaunchInteractiveSSHSession(host, keyLabel string) error {
	meta, keyPath, err := config.PrivateKeyPath(keyLabel)
	if err != nil {
		return err
	}
	if strings.TrimSpace(meta.DefaultUser) == "" {
		return fmt.Errorf("key %q has no default user; set one in SSH Keys before launching an interactive session", keyLabel)
	}

	hostPart, port, err := splitSSHHost(host)
	if err != nil {
		return err
	}

	sshArgs := []string{
		"-i", keyPath,
		"-o", "IdentitiesOnly=yes",
		"-o", "PreferredAuthentications=publickey",
		"-o", "PubkeyAuthentication=yes",
		"-o", "PasswordAuthentication=no",
		"-o", "KbdInteractiveAuthentication=no",
	}
	if port != "" && port != "22" {
		sshArgs = append(sshArgs, "-p", port)
	}
	sshArgs = append(sshArgs, meta.DefaultUser+"@"+hostPart)

	cmd, err := nativeTerminalCommand(sshArgs)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launching interactive SSH session: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}

func splitSSHHost(host string) (string, string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", "", fmt.Errorf("host is required")
	}

	if strings.HasPrefix(host, "[") {
		if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
			if err := validateSSHPort(parsedPort, host); err != nil {
				return "", "", err
			}
			return parsedHost, parsedPort, nil
		}
		if strings.HasSuffix(host, "]") {
			return strings.TrimSuffix(strings.TrimPrefix(host, "["), "]"), "22", nil
		}
		return "", "", fmt.Errorf("invalid host %q", host)
	}

	if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
		if err := validateSSHPort(parsedPort, host); err != nil {
			return "", "", err
		}
		return parsedHost, parsedPort, nil
	}

	if strings.Count(host, ":") == 1 {
		hostPart, portPart, _ := strings.Cut(host, ":")
		if hostPart == "" {
			return "", "", fmt.Errorf("host is required")
		}
		if err := validateSSHPort(portPart, host); err != nil {
			return "", "", err
		}
		return hostPart, portPart, nil
	}

	if strings.Count(host, ":") > 1 {
		if ip := net.ParseIP(host); ip == nil {
			return "", "", fmt.Errorf("invalid host %q", host)
		}
	}

	return host, "22", nil
}

func validateSSHPort(port, host string) error {
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("invalid port in host %q", host)
	}
	return nil
}

func nativeTerminalCommand(sshArgs []string) (*exec.Cmd, error) {
	switch goRuntime.GOOS {
	case "windows":
		return windowsTerminalCommand(sshArgs)
	case "linux":
		return linuxTerminalCommand(sshArgs)
	case "darwin":
		return macOSTerminalCommand(sshArgs)
	default:
		return nil, fmt.Errorf("interactive SSH launching is not supported on %s", goRuntime.GOOS)
	}
}

func windowsTerminalCommand(sshArgs []string) (*exec.Cmd, error) {
	if wtPath, err := exec.LookPath("wt.exe"); err == nil {
		return exec.Command(wtPath, append([]string{"ssh"}, sshArgs...)...), nil
	}

	if psPath, err := exec.LookPath("powershell.exe"); err == nil {
		return exec.Command(psPath, "-NoExit", "-Command", powershellSSHCommand(sshArgs)), nil
	}

	if cmdPath, err := exec.LookPath("cmd.exe"); err == nil {
		return exec.Command(cmdPath, "/k", cmdSSHCommandLine(sshArgs)), nil
	}

	return nil, fmt.Errorf("no supported Windows terminal launcher found (tried wt.exe, powershell.exe, cmd.exe)")
}

func linuxTerminalCommand(sshArgs []string) (*exec.Cmd, error) {
	launchers := []struct {
		program string
		args    func([]string) []string
	}{
		{
			program: "x-terminal-emulator",
			args: func(cmd []string) []string {
				return append([]string{"-e", "ssh"}, cmd...)
			},
		},
		{
			program: "gnome-terminal",
			args: func(cmd []string) []string {
				return append([]string{"--", "ssh"}, cmd...)
			},
		},
		{
			program: "konsole",
			args: func(cmd []string) []string {
				return append([]string{"-e", "ssh"}, cmd...)
			},
		},
		{
			program: "xfce4-terminal",
			args: func(cmd []string) []string {
				return []string{"--command", "ssh " + shellJoin(cmd)}
			},
		},
		{
			program: "xterm",
			args: func(cmd []string) []string {
				return append([]string{"-e", "ssh"}, cmd...)
			},
		},
	}

	for _, launcher := range launchers {
		path, err := exec.LookPath(launcher.program)
		if err == nil {
			return exec.Command(path, launcher.args(sshArgs)...), nil
		}
	}

	return nil, fmt.Errorf("no supported Linux terminal launcher found (tried x-terminal-emulator, gnome-terminal, konsole, xfce4-terminal, xterm)")
}

func macOSTerminalCommand(sshArgs []string) (*exec.Cmd, error) {
	osascriptPath, err := exec.LookPath("osascript")
	if err != nil {
		return nil, fmt.Errorf("osascript is required to launch Terminal on macOS: %w", err)
	}

	script := fmt.Sprintf(
		`tell application "Terminal"
activate
do script %s
end tell`,
		strconv.Quote("ssh "+shellJoin(sshArgs)),
	)
	return exec.Command(osascriptPath, "-e", script), nil
}

func powershellSSHCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "''")+"'")
	}
	return "& ssh " + strings.Join(quoted, " ")
}

func cmdSSHCommandLine(args []string) string {
	quoted := make([]string, 0, len(args)+1)
	quoted = append(quoted, "ssh")
	for _, arg := range args {
		quoted = append(quoted, strconv.Quote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
