//go:build !windows

package workstation

import "os/exec"

func configureCmd(_ *exec.Cmd) {}
