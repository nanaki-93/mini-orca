//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package app

import (
	"fmt"
	"os/exec"
)

// The runner refuses to start where Mini-Orca cannot own descendants.
func configureCheckCommand(_ *exec.Cmd) error {
	return fmt.Errorf("project command execution is unavailable: descendant cancellation is unsupported on this platform")
}
