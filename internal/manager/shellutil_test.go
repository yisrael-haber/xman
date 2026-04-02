package manager

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPosixCaptureScriptCapturesCompoundCommandOutputAndExitCode(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "command output's.txt")
	script := PosixCaptureScript("echo first; echo second; exit 7", outPath)

	cmd := exec.Command("/bin/sh", "-c", script)
	err := cmd.Run()
	if err == nil {
		t.Fatal("Run() error = nil, want non-zero exit status")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Run() error = %T, want *exec.ExitError", err)
	}
	if exitErr.ExitCode() != 7 {
		t.Fatalf("exit code = %d, want %d", exitErr.ExitCode(), 7)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", outPath, err)
	}
	got := strings.TrimSpace(string(data))
	if got != "first\nsecond" {
		t.Fatalf("captured output = %q, want %q", got, "first\nsecond")
	}
}

func TestPosixCaptureArgsWrapsScriptForShC(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "wrapped output.txt")
	args := PosixCaptureArgs("echo wrapped", outPath)

	if !strings.HasPrefix(args, "-c ") {
		t.Fatalf("PosixCaptureArgs() = %q, want -c prefix", args)
	}
	if !strings.Contains(args, ShQuote(outPath)) {
		t.Fatalf("PosixCaptureArgs() = %q, want quoted outPath %q", args, ShQuote(outPath))
	}
	if !strings.Contains(args, "echo wrapped") {
		t.Fatalf("PosixCaptureArgs() = %q, want command text", args)
	}
}
