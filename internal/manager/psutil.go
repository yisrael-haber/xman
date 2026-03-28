package manager

import (
	"encoding/base64"
	"strings"
	"unicode/utf16"
)

// IsWindows reports whether guestOS identifies a Windows guest.
// Handles both VMX short IDs ("win10-64") and display names ("Microsoft Windows 10 (64-bit)").
func IsWindows(guestOS string) bool {
	lower := strings.ToLower(guestOS)
	return strings.HasPrefix(lower, "win") || strings.Contains(lower, "windows")
}

// WinPSExePath is the full path to powershell.exe inside Windows guests.
const WinPSExePath = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`

// WinPSCmdArgs builds a powershell.exe Arguments string (for vCenter's
// GuestProgramSpec.Arguments) that runs cmdLine via cmd /c, captures all
// output to outPath using Out-File, and exits with the command's exit code.
//
// Using -EncodedCommand (UTF-16LE base64) sidesteps all quoting problems and
// avoids relying on cmd.exe's I/O redirection, which doesn't work reliably in
// VMware's headless guest-exec sessions.
func WinPSCmdArgs(cmdLine, outPath string) string {
	return "-NonInteractive -EncodedCommand " + encodePSScript(winPSCmdScript(cmdLine, outPath, ""))
}

// WinPSCmdArgList is the vmrun variant of WinPSCmdArgs. It returns the flags
// as separate strings so exec.Command passes them as distinct arguments to
// vmrun, which in turn passes each one separately to powershell.exe in the
// guest. Passing them as a single string causes vmrun to re-quote the whole
// thing, making PowerShell treat it as a positional parameter instead of flags.
func WinPSCmdArgList(cmdLine, outPath string) []string {
	return []string{"-NonInteractive", "-EncodedCommand", encodePSScript(winPSCmdScript(cmdLine, outPath, ""))}
}

// WinPSCmdArgsWithPID is the PowerShell encoded-command variant that also
// records the guest-side PowerShell PID to pidPath for later best-effort
// cancellation.
func WinPSCmdArgsWithPID(cmdLine, outPath, pidPath string) string {
	return "-NonInteractive -EncodedCommand " + encodePSScript(winPSCmdScript(cmdLine, outPath, pidPath))
}

// WinPSCmdArgListWithPID is the vmrun variant of WinPSCmdArgsWithPID.
func WinPSCmdArgListWithPID(cmdLine, outPath, pidPath string) []string {
	return []string{"-NonInteractive", "-EncodedCommand", encodePSScript(winPSCmdScript(cmdLine, outPath, pidPath))}
}

// WinPSStopPIDFromFileArgList returns a PowerShell encoded command that reads a
// PID from pidPath and forcefully stops that process if it exists.
func WinPSStopPIDFromFileArgList(pidPath string) []string {
	safePID := strings.ReplaceAll(pidPath, "'", "''")
	script := "if (Test-Path '" + safePID + "') {\n" +
		"    $raw = Get-Content -Path '" + safePID + "' -ErrorAction SilentlyContinue | Select-Object -First 1\n" +
		"    if ($raw) {\n" +
		"        $pidValue = 0\n" +
		"        if ([int]::TryParse($raw.ToString(), [ref]$pidValue)) {\n" +
		"            Stop-Process -Id $pidValue -Force -ErrorAction SilentlyContinue\n" +
		"        }\n" +
		"    }\n" +
		"}"
	return []string{"-NonInteractive", "-EncodedCommand", encodePSScript(script)}
}

func winPSCmdScript(cmdLine, outPath, pidPath string) string {
	safeCmd := strings.ReplaceAll(cmdLine, "'", "''")
	safeOut := strings.ReplaceAll(outPath, "'", "''")
	safePID := strings.ReplaceAll(pidPath, "'", "''")
	script := "$ErrorActionPreference = 'Continue'\n"
	if safePID != "" {
		script += "Set-Content -Path '" + safePID + "' -Value $PID -Encoding ASCII -Force\n"
	}
	script +=
		"try {\n" +
			"    cmd /c '" + safeCmd + "' 2>&1 | Out-File -FilePath '" + safeOut + "' -Encoding UTF8 -Force\n" +
			"} catch {\n" +
			"    \"ERROR: $_\" | Out-File -FilePath '" + safeOut + "' -Encoding UTF8 -Force\n" +
			"}\n" +
			"exit $LASTEXITCODE"
	return script
}

// encodePSScript encodes a PowerShell script as UTF-16LE base64 for use with
// powershell.exe -EncodedCommand.
func encodePSScript(script string) string {
	utf16le := utf16.Encode([]rune(script))
	b := make([]byte, len(utf16le)*2)
	for i, r := range utf16le {
		b[i*2] = byte(r)
		b[i*2+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}
