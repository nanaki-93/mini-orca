//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package llm

import (
	"fmt"
	"os/exec"
)

func configureCLIProcess(*exec.Cmd) error {
	return fmt.Errorf("llm client: %w: CLI providers require a platform with process-group cleanup", ErrRequestRejected)
}
