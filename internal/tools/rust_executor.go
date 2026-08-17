package tools

import (
	"context"
	"fmt"
)

// rustExecutor implements Rust-specific operations.
type rustExecutor struct {
	shell Executor
}

// NewRustExecutor creates a new Rust executor instance.
func NewRustExecutor() *rustExecutor {
	return newRustExecutorAt("")
}

func newRustExecutorAt(workingDir string) *rustExecutor {
	return &rustExecutor{
		shell: newSafeShellExecutorAt(workingDir),
	}
}

// FormatCode runs `cargo fmt` to format Rust code.
func (r *rustExecutor) FormatCode() error {
	_, err := r.shell.Shell(context.Background(), "cargo", "fmt")
	if err != nil {
		return fmt.Errorf("rust_executor: cargo fmt failed: %w", err)
	}
	return nil
}

// RunTests runs `cargo test` to execute Rust tests.
func (r *rustExecutor) RunTests() (*ShellResult, error) {
	result, err := r.shell.Shell(context.Background(), "cargo", "test")
	if err != nil {
		return result, fmt.Errorf("rust_executor: cargo test failed: %w", err)
	}
	return result, nil
}

// Build runs `cargo build` to build the Rust project.
func (r *rustExecutor) Build() error {
	_, err := r.shell.Shell(context.Background(), "cargo", "build")
	if err != nil {
		return fmt.Errorf("rust_executor: cargo build failed: %w", err)
	}
	return nil
}

// Lint runs `cargo clippy` to lint Rust code.
func (r *rustExecutor) Lint() (*ShellResult, error) {
	return r.shell.Shell(context.Background(), "cargo", "clippy")
}
