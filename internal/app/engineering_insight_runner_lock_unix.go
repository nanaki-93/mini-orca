//go:build !windows

package app

import (
	"fmt"
	"os"
	"syscall"
)

// runnerLockedFile uses advisory OS locks, which are released automatically
// when a crashed process exits while retaining the source-free lock file.
type runnerLockedFile struct{ file *os.File }

func lockRunnerFile(path string) (*runnerLockedFile, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("lock evaluation run")
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, ErrEngineeringInsightRunLocked
	}
	return &runnerLockedFile{file: file}, nil
}

func (lock *runnerLockedFile) Close() {
	_ = syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	_ = lock.file.Close()
}
