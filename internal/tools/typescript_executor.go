package tools

import (
	"context"
	"fmt"
)

// typescriptExecutor implements TypeScript-specific operations.
type typescriptExecutor struct {
	shell Executor
}

// NewTypeScriptExecutor creates a new TypeScript executor instance.
func NewTypeScriptExecutor() *typescriptExecutor {
	return &typescriptExecutor{
		shell: NewSafeShellExecutor(),
	}
}

// FormatCode runs `npx prettier --write` to format TypeScript code.
func (t *typescriptExecutor) FormatCode() error {
	_, err := t.shell.Shell(context.Background(), "npx", "prettier", "--write", ".")
	if err != nil {
		return fmt.Errorf("typescript_executor: npx prettier failed: %w", err)
	}
	return nil
}

// RunTests runs `npx jest` to execute TypeScript tests.
func (t *typescriptExecutor) RunTests() (*ShellResult, error) {
	result, err := t.shell.Shell(context.Background(), "npx", "jest")
	if err != nil {
		return result, fmt.Errorf("typescript_executor: npx jest failed: %w", err)
	}
	return result, nil
}

// Build runs `npx tsc --noEmit` to type-check the TypeScript project.
func (t *typescriptExecutor) Build() error {
	_, err := t.shell.Shell(context.Background(), "npx", "tsc", "--noEmit")
	if err != nil {
		return fmt.Errorf("typescript_executor: npx tsc failed: %w", err)
	}
	return nil
}

// Lint runs `npx eslint` to lint TypeScript code.
func (t *typescriptExecutor) Lint() (*ShellResult, error) {
	return t.shell.Shell(context.Background(), "npx", "eslint", ".")
}
