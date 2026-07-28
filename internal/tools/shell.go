package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// shellExecutor implements the Executor interface for shell command execution.
type shellExecutor struct{}

// NewShellExecutor creates a new shell executor instance.
func NewShellExecutor() Executor {
	return &shellExecutor{}
}

// Shell executes a shell command with the given arguments and returns the result.
func (s *shellExecutor) Shell(ctx context.Context, command string, args ...string) (*ShellResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return nil, fmt.Errorf("shell: command execution failed: %w", err)
		}
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = strings.TrimSpace(stderr.String())
	}

	return &ShellResult{
		Output:   output,
		ExitCode: exitCode,
		Err:      err,
	}, nil
}

// ReadFile reads the content of a file at the given path.
func (s *shellExecutor) ReadFile(path string) ([]byte, error) {
	return nil, fmt.Errorf("shell: ReadFile not implemented")
}

// WriteFile writes content to a file at the given path.
func (s *shellExecutor) WriteFile(path string, data []byte, perm uint32) error {
	return fmt.Errorf("shell: WriteFile not implemented")
}

// DetectProjectType detects the type of project at the given directory.
func (s *shellExecutor) DetectProjectType(dirPath string) (*ProjectInfo, error) {
	return nil, fmt.Errorf("shell: DetectProjectType not implemented")
}

// Format formats code content using the appropriate formatter.
func (s *shellExecutor) Format(ctx context.Context, content []byte, language string) ([]byte, error) {
	return nil, fmt.Errorf("shell: Format not implemented")
}
