package tools

import (
	"context"
	"fmt"
	"os/exec"
)

// goExecutor implements Go-specific operations.
type goExecutor struct {
	shell Executor
}

// NewGoExecutor creates a new Go executor instance.
func NewGoExecutor() *goExecutor {
	return &goExecutor{
		shell: NewShellExecutor(),
	}
}

// FormatCode runs `go fmt ./...` to format Go code.
func (g *goExecutor) FormatCode() error {
	_, err := g.shell.Shell(context.Background(), "go", "fmt", "./...")
	if err != nil {
		return fmt.Errorf("go_executor: go fmt failed: %w", err)
	}
	return nil
}

// RunTests runs `go test ./... -v -cover` to execute Go tests.
func (g *goExecutor) RunTests() (*ShellResult, error) {
	result, err := g.shell.Shell(context.Background(), "go", "test", "./...", "-v", "-cover")
	if err != nil {
		return result, fmt.Errorf("go_executor: go test failed: %w", err)
	}
	return result, nil
}

// Build runs `go build ./...` to build the Go project.
func (g *goExecutor) Build() error {
	_, err := g.shell.Shell(context.Background(), "go", "build", "./...")
	if err != nil {
		return fmt.Errorf("go_executor: go build failed: %w", err)
	}
	return nil
}

// Lint runs `golangci-lint run` if available, falling back to `go vet`.
func (g *goExecutor) Lint() (*ShellResult, error) {
	if g.isCommandAvailable("golangci-lint") {
		return g.shell.Shell(context.Background(), "golangci-lint", "run")
	}
	return g.shell.Shell(context.Background(), "go", "vet", "./...")
}

// isCommandAvailable checks if a command exists in PATH.
func (g *goExecutor) isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
