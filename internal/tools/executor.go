package tools

import (
	"context"
	"time"
)

// Executor defines the interface for executing tools and operations.
// This abstraction enables dependency inversion, allowing different
// executor implementations to be swapped without modifying dependents.
type Executor interface {
	// Shell executes a shell command and returns its output.
	Shell(ctx context.Context, command string, args ...string) (*ShellResult, error)

	// ReadFile reads the content of a file at the given path.
	ReadFile(path string) ([]byte, error)

	// WriteFile writes content to a file at the given path.
	WriteFile(path string, data []byte, perm uint32) error

	// DetectProjectType detects the type of project at the given directory.
	DetectProjectType(dirPath string) (*ProjectInfo, error)

	// Format formats code content using the appropriate formatter.
	Format(ctx context.Context, content []byte, language string) ([]byte, error)
}

// ShellResult holds the output from a shell command execution.
type ShellResult struct {
	// Output contains the combined stdout and stderr.
	Output string
	// ExitCode is the exit code of the command.
	ExitCode int
	// Err contains any error that occurred during execution.
	Err error
}

// ToolExecutor defines the interface for executing tools and performing file operations.
// This abstraction enables dependency inversion, allowing different
// executor implementations to be swapped without modifying dependents.
type ToolExecutor interface {
	// Execute runs a command with the given arguments and timeout.
	Execute(cmd string, args []string, timeout time.Duration) (*ExecResult, error)

	// FormatCode formats the code file at the given path.
	FormatCode(path string) error

	// WriteFile writes content to a file at the given path.
	WriteFile(path string, content string) error

	// ReadFile reads the content of a file at the given path.
	ReadFile(path string) (string, error)

	// AppendToFile appends content to a file at the given path.
	AppendToFile(path string, content string) error
}

// ExecResult holds the result of a command execution.
type ExecResult struct {
	// ExitCode is the exit code of the command.
	ExitCode int
	// Stdout contains the standard output.
	Stdout string
	// Stderr contains the standard error.
	Stderr string
	// Duration is the time taken to execute the command.
	Duration time.Duration
}
