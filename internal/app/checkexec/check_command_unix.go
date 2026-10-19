//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package checkexec

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func ConfigureCommand(process *exec.Cmd) error {
	process.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	process.Cancel = func() error {
		if process.Process == nil {
			return nil
		}
		err := syscall.Kill(-process.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return nil
}
