//go:build linux
// +build linux

package framework

import (
	"os/exec"
	"syscall"
)

func execCommand(workdir, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(binaryName, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
	cmd.Dir = workdir

	return cmd
}

func processKill(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
