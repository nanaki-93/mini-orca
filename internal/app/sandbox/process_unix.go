//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package sandbox

import (
	"errors"
	"fmt"
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

// VerifyCommandGroupStopped proves absence after the owner has reaped its
// leader. A failed signal alone never establishes successful cleanup.
func VerifyCommandGroupStopped(process *exec.Cmd) error {
	if process == nil || process.Process == nil || process.ProcessState == nil || process.SysProcAttr == nil || !process.SysProcAttr.Setpgid {
		return fmt.Errorf("owned process group has not been reaped")
	}
	err := syscall.Kill(-process.Process.Pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("verify process group cleanup: %w", err)
	}
	return fmt.Errorf("owned process group still exists")
}
