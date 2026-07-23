package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ShellExecutor provides universal shell command execution with timeout and context support.
type ShellExecutor struct {
	timeout time.Duration
}

// NewShellExecutor creates a new shell executor with the given timeout.
func NewShellExecutor(timeout time.Duration) *ShellExecutor {
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &ShellExecutor{
		timeout: timeout,
	}
}

// Execute runs a shell command with the given working directory.
func (s *ShellExecutor) Execute(ctx context.Context, workDir, command string, args ...string) (*CommandOutput, error) {
	fullCmd := exec.CommandContext(ctx, command, args...)
	fullCmd.Dir = workDir

	output, err := fullCmd.CombinedOutput()

	return &CommandOutput{
		Stdout: string(output),
		Stderr: string(output),
		Success: err == nil,
	}, err
}

// RunWithTimeout runs a command with a specific timeout.
func (s *ShellExecutor) RunWithTimeout(workDir, command string, timeout time.Duration, args ...string) (*CommandOutput, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return s.Execute(ctx, workDir, command, args...)
}

// CommandOutput holds the result of a shell command execution.
type CommandOutput struct {
	Stdout  string
	Stderr  string
	Success bool
}

// CombinedOutput returns both stdout and stderr combined.
func (o *CommandOutput) CombinedOutput() string {
	if o.Stdout != "" && o.Stderr != "" && !strings.Contains(o.Stdout, o.Stderr) {
		return o.Stdout + "\n" + o.Stderr
	}
	return o.Stdout + o.Stderr
}

// Error returns an error string if the command failed.
func (o *CommandOutput) Error() string {
	if o.Success {
		return ""
	}
	return fmt.Sprintf("command failed with output: %s", o.CombinedOutput())
}

// ShortOutput returns the first 1000 characters of output for logging.
func (o *CommandOutput) ShortOutput() string {
	output := o.CombinedOutput()
	if len(output) > 1000 {
		return output[:1000] + "... [truncated]"
	}
	return output
}
