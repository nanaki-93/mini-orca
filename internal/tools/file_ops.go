package tools

import (
	"context"
	"fmt"
	"os"
)

// fileOpsExecutor implements file operations for the Executor interface.
type fileOpsExecutor struct {
	shell Executor
}

// NewFileOpsExecutor creates a new file operations executor instance.
func NewFileOpsExecutor() Executor {
	return &fileOpsExecutor{
		shell: NewShellExecutor(),
	}
}

// _ ensures NewShellExecutor is used.
var _ = NewShellExecutor

// ReadFile reads the content of a file at the given path.
func (f *fileOpsExecutor) ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("file_ops: failed to read file %s: %w", path, err)
	}
	return data, nil
}

// WriteFile writes content to a file at the given path with the specified permissions.
func (f *fileOpsExecutor) WriteFile(path string, data []byte, perm uint32) error {
	err := os.WriteFile(path, data, os.FileMode(perm))
	if err != nil {
		return fmt.Errorf("file_ops: failed to write file %s: %w", path, err)
	}
	return nil
}

// Shell executes a shell command and returns its output.
func (f *fileOpsExecutor) Shell(ctx context.Context, command string, args ...string) (*ShellResult, error) {
	return f.shell.Shell(ctx, command, args...)
}

// DetectProjectType detects the type of project at the given directory.
func (f *fileOpsExecutor) DetectProjectType(dirPath string) (*ProjectInfo, error) {
	return f.shell.DetectProjectType(dirPath)
}

// Format formats code content using the appropriate formatter.
func (f *fileOpsExecutor) Format(ctx context.Context, content []byte, language string) ([]byte, error) {
	return f.shell.Format(ctx, content, language)
}
