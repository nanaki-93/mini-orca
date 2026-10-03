package llm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

var errCLIOutputLimit = errors.New("CLI output exceeded its byte limit")

type cliOutputBuffer struct {
	bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	exceeded bool
}

func (b *cliOutputBuffer) Write(data []byte) (int, error) {
	if len(data) > b.limit-b.Len() {
		b.exceeded = true
		b.cancel()
		return 0, errCLIOutputLimit
	}
	return b.Buffer.Write(data)
}

func runCLI(ctx context.Context, profile config.ModelProfile, invocation cliInvocation) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	executable, err := exec.LookPath(profile.CLIPath)
	if err != nil {
		return nil, fmt.Errorf("llm client: %w: %s CLI executable is unavailable; install it on the daemon's PATH or set cli_path", ErrRequestRejected, profile.Provider)
	}
	timed, cancel := context.WithTimeout(ctx, providerRequestTimeout)
	defer cancel()
	process := exec.CommandContext(timed, executable, invocation.args...)
	process.Dir = invocation.directory
	process.Stdin = strings.NewReader(invocation.input)
	stdout := &cliOutputBuffer{limit: maxProviderResponseBytes, cancel: cancel}
	stderr := &cliOutputBuffer{limit: 64 * 1024, cancel: cancel}
	process.Stdout, process.Stderr = stdout, stderr
	process.WaitDelay = time.Second
	if err := configureCLIProcess(process); err != nil {
		return nil, err
	}
	err = process.Run()
	// Also stop inherited children after a normal exit; they cannot outlive a request.
	if process.Process != nil {
		if cleanupErr := process.Cancel(); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrProcessDone) {
			return nil, fmt.Errorf("llm client: CLI process-group cleanup failed")
		}
	}
	if stdout.exceeded || stderr.exceeded {
		return nil, fmt.Errorf("llm client: %w: CLI output exceeds byte limit", ErrUnusableResponse)
	}
	if timed.Err() != nil {
		return nil, fmt.Errorf("llm client: CLI request ended: %w", timed.Err())
	}
	if err != nil {
		return nil, cliProcessError(profile.Provider, err)
	}
	return stdout.Bytes(), nil
}

func cliProcessError(provider config.ModelProvider, err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return fmt.Errorf("llm client: %s CLI exited with status %d; check its login, model, and supported CLI options", provider, exit.ExitCode())
	}
	// Subprocess errors and stderr may contain source, credentials or private paths.
	return fmt.Errorf("llm client: %w: %s CLI could not complete; check its installation and permissions", ErrRequestRejected, provider)
}
