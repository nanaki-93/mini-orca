//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package llm

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
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
	for attempt := 1; ; attempt++ {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, syscall.ESRCH):
			return os.ErrProcessDone
		case !errors.Is(err, syscall.EPERM):
			return err
		case attempt < zombieGroupAttempts:
			// macOS refuses to signal a group while killed members wait, as
			// zombies, to be reaped by launchd; they cannot run. Try again.
			time.Sleep(25 * time.Millisecond)
		case command.ProcessState != nil:
			// The CLI was waited for and every member was signaled: what is
			// left are zombies.
			return os.ErrProcessDone
		default:
			return err
		}
	}
}

// zombieGroupAttempts bounds the wait for a killed group's zombies (half a second).
const zombieGroupAttempts = 20
