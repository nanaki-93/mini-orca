package tools

import (
	"context"
	"fmt"
	"os/exec"
)

// pythonExecutor implements Python-specific operations.
type pythonExecutor struct {
	shell Executor
}

// NewPythonExecutor creates a new Python executor instance.
func NewPythonExecutor() *pythonExecutor {
	return newPythonExecutorAt("")
}

func newPythonExecutorAt(workingDir string) *pythonExecutor {
	return &pythonExecutor{
		shell: newSafeShellExecutorAt(workingDir),
	}
}

// FormatCode runs `black .` to format Python code.
func (p *pythonExecutor) FormatCode() error {
	_, err := p.shell.Shell(context.Background(), "black", ".")
	if err != nil {
		return fmt.Errorf("python_executor: black failed: %w", err)
	}
	return nil
}

// RunTests runs `pytest` to execute Python tests.
func (p *pythonExecutor) RunTests() (*ShellResult, error) {
	result, err := p.shell.Shell(context.Background(), "pytest")
	if err != nil {
		return result, fmt.Errorf("python_executor: pytest failed: %w", err)
	}
	return result, nil
}

// Build runs `python -m py_compile` to compile Python files.
func (p *pythonExecutor) Build() error {
	_, err := p.shell.Shell(context.Background(), "python", "-m", "py_compile", ".")
	if err != nil {
		return fmt.Errorf("python_executor: py_compile failed: %w", err)
	}
	return nil
}

// Lint runs `flake8` if available, falling back to `pylint`.
func (p *pythonExecutor) Lint() (*ShellResult, error) {
	if p.isCommandAvailable("flake8") {
		return p.shell.Shell(context.Background(), "flake8", ".")
	}
	return p.shell.Shell(context.Background(), "pylint", ".")
}

// isCommandAvailable checks if a command exists in PATH.
func (p *pythonExecutor) isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
