package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// formatterExecutor implements code formatting for the Executor interface.
type formatterExecutor struct {
	fileOps Executor
}

// NewFormatterExecutor creates a new formatter executor instance.
func NewFormatterExecutor() Executor {
	return &formatterExecutor{
		fileOps: NewFileOpsExecutor(),
	}
}

// _ ensures NewFileOpsExecutor is used.
var _ = NewFileOpsExecutor

// Format formats code content using the appropriate formatter based on language.
func (f *formatterExecutor) Format(ctx context.Context, content []byte, language string) ([]byte, error) {
	switch language {
	case "go":
		return f.formatGo(ctx, content)
	default:
		return content, nil
	}
}

// formatGo formats Go code using gofmt.
func (f *formatterExecutor) formatGo(ctx context.Context, content []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gofmt")
	cmd.Stdin = bytes.NewReader(content)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("formatter: gofmt failed: %w, stderr: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// Shell executes a shell command and returns its output.
func (f *formatterExecutor) Shell(ctx context.Context, command string, args ...string) (*ShellResult, error) {
	return f.fileOps.Shell(ctx, command, args...)
}

// ReadFile reads the content of a file at the given path.
func (f *formatterExecutor) ReadFile(path string) ([]byte, error) {
	return f.fileOps.ReadFile(path)
}

// WriteFile writes content to a file at the given path with the specified permissions.
func (f *formatterExecutor) WriteFile(path string, data []byte, perm uint32) error {
	return f.fileOps.WriteFile(path, data, perm)
}

// DetectProjectType detects the type of project at the given directory.
func (f *formatterExecutor) DetectProjectType(dirPath string) (*ProjectInfo, error) {
	return f.fileOps.DetectProjectType(dirPath)
}
