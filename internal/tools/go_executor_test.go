package tools

import (
	"context"
	"testing"
	"time"
)

type MockExecutor struct {
	shellFunc func(ctx context.Context, command string, args ...string) (*ShellResult, error)
}

func (m *MockExecutor) Shell(ctx context.Context, command string, args ...string) (*ShellResult, error) {
	if m.shellFunc != nil {
		return m.shellFunc(ctx, command, args...)
	}
	return &ShellResult{}, nil
}
func (m *MockExecutor) ReadFile(path string) ([]byte, error)                   { return nil, nil }
func (m *MockExecutor) WriteFile(path string, data []byte, perm uint32) error  { return nil }
func (m *MockExecutor) DetectProjectType(dirPath string) (*ProjectInfo, error) { return nil, nil }
func (m *MockExecutor) Format(ctx context.Context, content []byte, language string) ([]byte, error) {
	return nil, nil
}
func (m *MockExecutor) FormatCode(path string) error { return nil }
func (m *MockExecutor) Execute(cmd string, args []string, timeout time.Duration) (*ExecResult, error) {
	return nil, nil
}

func TestGoExecutor(t *testing.T) {
	mock := &MockExecutor{}
	g := &goExecutor{shell: mock}

	t.Run("FormatCode", func(t *testing.T) {
		called := false
		mock.shellFunc = func(ctx context.Context, command string, args ...string) (*ShellResult, error) {
			called = true
			if command != "go" || args[0] != "fmt" {
				t.Errorf("unexpected command: %s %v", command, args)
			}
			return &ShellResult{}, nil
		}
		err := g.FormatCode()
		if err != nil || !called {
			t.Errorf("FormatCode failed: %v, called: %t", err, called)
		}
	})

	t.Run("RunTests", func(t *testing.T) {
		mock.shellFunc = func(ctx context.Context, command string, args ...string) (*ShellResult, error) {
			return &ShellResult{Output: "PASS"}, nil
		}
		res, err := g.RunTests()
		if err != nil || res.Output != "PASS" {
			t.Errorf("RunTests failed: %v, res: %v", err, res)
		}
	})

	t.Run("Build", func(t *testing.T) {
		mock.shellFunc = func(ctx context.Context, command string, args ...string) (*ShellResult, error) {
			return &ShellResult{}, nil
		}
		err := g.Build()
		if err != nil {
			t.Errorf("Build failed: %v", err)
		}
	})
}
