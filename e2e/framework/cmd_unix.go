//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos
// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris zos

package framework

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

var pids = []int{}

func init() {
	go func() {
		// wait os.Interrupt signal
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		for sig := range c {
			if sig == os.Interrupt {
				// kill the whole process group
				for _, pid := range pids {
					_ = syscall.Kill(-pid, syscall.SIGKILL)
				}
			}

			os.Exit(1)
		}
	}()
}

func registerPID(cmd *exec.Cmd) {
	pids = append(pids, cmd.Process.Pid)
}

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
	// kill the whole process group
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(syscall.ESRCH, err) {
		return nil
	}

	return err
}
