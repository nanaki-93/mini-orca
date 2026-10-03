//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package llm

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureCLIProcess(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return killCLIGroup(command) }
	return nil
}

func killCLIGroup(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}
