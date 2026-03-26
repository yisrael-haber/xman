package manager

import (
	"encoding/base64"
	"strings"
	"unicode/utf16"
)

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
	return "-NonInteractive -EncodedCommand " + encodePSScript(winPSCmdScript(cmdLine, outPath))
}

// WinPSCmdArgList is the vmrun variant of WinPSCmdArgs. It returns the flags
// as separate strings so exec.Command passes them as distinct arguments to
// vmrun, which in turn passes each one separately to powershell.exe in the
// guest. Passing them as a single string causes vmrun to re-quote the whole
// thing, making PowerShell treat it as a positional parameter instead of flags.
func WinPSCmdArgList(cmdLine, outPath string) []string {
	return []string{"-NonInteractive", "-EncodedCommand", encodePSScript(winPSCmdScript(cmdLine, outPath))}
}

func winPSCmdScript(cmdLine, outPath string) string {
	safeCmd := strings.ReplaceAll(cmdLine, "'", "''")
	safeOut := strings.ReplaceAll(outPath, "'", "''")
	return "$ErrorActionPreference = 'Continue'\n" +
		"try {\n" +
		"    cmd /c '" + safeCmd + "' 2>&1 | Out-File -FilePath '" + safeOut + "' -Encoding UTF8 -Force\n" +
		"} catch {\n" +
		"    \"ERROR: $_\" | Out-File -FilePath '" + safeOut + "' -Encoding UTF8 -Force\n" +
		"}\n" +
		"exit $LASTEXITCODE"
}

// WinPSExprArgs builds a powershell.exe Arguments string (for vCenter's
// GuestProgramSpec.Arguments) that evaluates psExpr as a PowerShell
// expression, captures all output to outPath, and exits with $LASTEXITCODE.
func WinPSExprArgs(psExpr, outPath string) string {
	return "-NonInteractive -EncodedCommand " + encodePSScript(winPSExprScript(psExpr, outPath))
}

func winPSExprScript(psExpr, outPath string) string {
	safeExpr := strings.ReplaceAll(psExpr, "'", "''")
	safeOut := strings.ReplaceAll(outPath, "'", "''")
	return "$ErrorActionPreference = 'Continue'\n" +
		"try {\n" +
		"    " + safeExpr + " 2>&1 | Out-File -FilePath '" + safeOut + "' -Encoding UTF8 -Force\n" +
		"} catch {\n" +
		"    \"ERROR: $_\" | Out-File -FilePath '" + safeOut + "' -Encoding UTF8 -Force\n" +
		"}\n" +
		"exit $LASTEXITCODE"
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
