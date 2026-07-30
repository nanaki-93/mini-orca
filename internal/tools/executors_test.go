package tools

import (
	"context"
	"testing"
)

func TestJavaExecutor(t *testing.T) {
	mock := &MockExecutor{}
	e := &javaExecutor{shell: mock}

	t.Run("FormatCode", func(t *testing.T) {
		mock.shellFunc = func(ctx context.Context, command string, args ...string) (*ShellResult, error) {
			return &ShellResult{}, nil
		}
		err := e.FormatCode()
		if err != nil {
			t.Errorf("FormatCode failed: %v", err)
		}
	})

	t.Run("RunTests", func(t *testing.T) {
		mock.shellFunc = func(ctx context.Context, command string, args ...string) (*ShellResult, error) {
			return &ShellResult{}, nil
		}
		_, err := e.RunTests()
		if err != nil {
			t.Errorf("RunTests failed: %v", err)
		}
	})
}

func TestKotlinExecutor(t *testing.T) {
	mock := &MockExecutor{}
	e := &kotlinExecutor{shell: mock}

	t.Run("FormatCode", func(t *testing.T) {
		mock.shellFunc = func(ctx context.Context, command string, args ...string) (*ShellResult, error) {
			return &ShellResult{}, nil
		}
		err := e.FormatCode()
		if err != nil {
			t.Errorf("FormatCode failed: %v", err)
		}
	})
}

func TestRustExecutor(t *testing.T) {
	mock := &MockExecutor{}
	e := &rustExecutor{shell: mock}

	t.Run("FormatCode", func(t *testing.T) {
		mock.shellFunc = func(ctx context.Context, command string, args ...string) (*ShellResult, error) {
			return &ShellResult{}, nil
		}
		err := e.FormatCode()
		if err != nil {
			t.Errorf("FormatCode failed: %v", err)
		}
	})
}

func TestPythonExecutor(t *testing.T) {
	mock := &MockExecutor{}
	e := &pythonExecutor{shell: mock}

	t.Run("FormatCode", func(t *testing.T) {
		mock.shellFunc = func(ctx context.Context, command string, args ...string) (*ShellResult, error) {
			return &ShellResult{}, nil
		}
		err := e.FormatCode()
		if err != nil {
			t.Errorf("FormatCode failed: %v", err)
		}
	})
}

func TestTypeScriptExecutor(t *testing.T) {
	mock := &MockExecutor{}
	e := &typescriptExecutor{shell: mock}

	t.Run("FormatCode", func(t *testing.T) {
		mock.shellFunc = func(ctx context.Context, command string, args ...string) (*ShellResult, error) {
			return &ShellResult{}, nil
		}
		err := e.FormatCode()
		if err != nil {
			t.Errorf("FormatCode failed: %v", err)
		}
	})
}
