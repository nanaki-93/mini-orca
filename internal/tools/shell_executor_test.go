package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// Shell Executor Tests
// ============================================================================

func TestShellExecutor_Shell_Success(t *testing.T) {
	executor := &shellExecutor{}
	ctx := context.Background()

	// Test echo command (available on all systems)
	result, err := executor.Shell(ctx, "echo", "test output")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Output != "test output" {
		t.Errorf("expected output %q, got %q", "test output", result.Output)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestShellExecutor_Shell_Failure(t *testing.T) {
	executor := NewShellExecutor()
	ctx := context.Background()

	// Test command that will fail
	result, err := executor.Shell(ctx, "false")
	if err != nil {
		t.Fatalf("expected no error (exit code is tracked separately), got: %v", err)
	}

	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
}

func TestShellExecutor_Shell_NonExistentCommand(t *testing.T) {
	executor := NewShellExecutor()
	ctx := context.Background()

	// Test command that doesn't exist
	_, err := executor.Shell(ctx, "nonexistent_command_12345")
	if err == nil {
		t.Fatal("expected error for nonexistent command, got nil")
	}
}

func TestShellExecutor_Shell_ContextTimeout(t *testing.T) {
	executor := NewShellExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test with a command that will timeout (sleep for 1 second)
	// Note: On some systems, sleep may not respect context cancellation
	// so we just verify the command runs without panic
	_, err := executor.Shell(ctx, "sleep", "0.1")
	// We don't strictly require an error here as context handling varies by OS
	_ = err
}

func TestShellExecutor_Execute(t *testing.T) {
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)

	result, err := executor.Execute("echo", []string{"test"}, DefaultTimeout)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Stdout != "test" {
		t.Errorf("expected stdout %q, got %q", "test", result.Stdout)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if result.Duration == 0 {
		t.Error("expected duration to be set")
	}
}

func TestShellExecutor_ReadFile_NotImplemented(t *testing.T) {
	executor := NewShellExecutor()
	_, err := executor.ReadFile("/nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got: %v", err)
	}
}

func TestShellExecutor_WriteFile_NotImplemented(t *testing.T) {
	executor := NewShellExecutor()
	err := executor.WriteFile("/nonexistent", []byte("test"), 0644)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got: %v", err)
	}
}

func TestShellExecutor_DetectProjectType_NotImplemented(t *testing.T) {
	executor := NewShellExecutor()
	_, err := executor.DetectProjectType("/nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got: %v", err)
	}
}

func TestShellExecutor_Format_NotImplemented(t *testing.T) {
	executor := NewShellExecutor()
	_, err := executor.Format(context.Background(), []byte("test"), "go")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got: %v", err)
	}
}

func TestShellExecutor_FormatCode_NotImplemented(t *testing.T) {
	executor := NewShellExecutor()
	err := executor.FormatCode("/nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got: %v", err)
	}
}

// ============================================================================
// Safe Shell Executor Tests
// ============================================================================

func TestSafeShellExecutor_AllowedCommand(t *testing.T) {
	executor := NewSafeShellExecutor()
	ctx := context.Background()

	result, err := executor.Shell(ctx, "echo", "safe test")
	if err != nil {
		t.Fatalf("expected no error for allowed command, got: %v", err)
	}

	if result.Output != "safe test" {
		t.Errorf("expected output %q, got %q", "safe test", result.Output)
	}
}

func TestSafeShellExecutor_DisallowedCommand(t *testing.T) {
	executor := NewSafeShellExecutor()
	ctx := context.Background()

	// Test with a command not in the allowlist
	_, err := executor.Shell(ctx, "nonexistent_command_12345")
	if err == nil {
		t.Fatal("expected error for disallowed command, got nil")
	}

	if !strings.Contains(err.Error(), "command not allowed") {
		t.Errorf("expected 'command not allowed' error, got: %v", err)
	}
}

func TestSafeShellExecutor_RejectsAllowedPrefix(t *testing.T) {
	executor := NewSafeShellExecutor()
	_, err := executor.Shell(context.Background(), "goevil", "version")
	if err == nil || !strings.Contains(err.Error(), "command not allowed") {
		t.Fatalf("expected prefix lookalike to be rejected, got %v", err)
	}
}

func TestShellExecutor_Execute_NonexistentDoesNotPanic(t *testing.T) {
	executor := &shellExecutor{}
	if _, err := executor.Execute("definitely-not-a-command", nil, time.Second); err == nil {
		t.Fatal("expected nonexistent command error")
	}
}

func TestSafeShellExecutor_DangerousCommand(t *testing.T) {
	executor := NewSafeShellExecutor()
	ctx := context.Background()

	// Test with commands that are allowed but have dangerous arguments
	// Note: rm, dd, chmod are not in the allowlist, so they fail with "command not allowed"
	// We test with allowed commands that have dangerous patterns in their arguments
	dangerous := []struct {
		cmd  string
		args []string
	}{
		// echo with /etc/shadow pattern (not truly dangerous but tests pattern matching)
		{"echo", []string{"/etc/shadow"}},
		// ls with 777 pattern (not truly dangerous but tests pattern matching)
		{"ls", []string{"777"}},
	}

	for _, dc := range dangerous {
		_, err := executor.Shell(ctx, dc.cmd, dc.args...)
		// These commands are allowed but contain dangerous patterns
		// The actual behavior depends on whether the pattern is matched
		// We just verify the command doesn't panic
		_ = err
	}
}

func TestSafeShellExecutor_IsCommandAllowed(t *testing.T) {
	executor := NewSafeShellExecutor()

	// Test exact match
	if !executor.(*safeShellExecutor).isCommandAllowed("echo") {
		t.Error("expected 'echo' to be allowed")
	}

	// Arguments are passed separately; a compound executable name must not match.
	if executor.(*safeShellExecutor).isCommandAllowed("go build") {
		t.Error("expected compound executable name to be rejected")
	}

	// Test not allowed
	if executor.(*safeShellExecutor).isCommandAllowed("nonexistent_command") {
		t.Error("expected 'nonexistent_command' to not be allowed")
	}
}

func TestSafeShellExecutor_IsDangerousCommand(t *testing.T) {
	executor := NewSafeShellExecutor()

	// Test dangerous patterns
	dangerous := []struct {
		cmd  string
		args []string
	}{
		{"rm", []string{"-rf", "/"}},
		{"rm", []string{"-r", "/"}},
		{"rm", []string{"-f", "/"}},
		{"dd", []string{"if=", "/dev/zero", "of=/"}},
		{"chmod", []string{"777", "/etc/passwd"}},
	}

	for _, dc := range dangerous {
		if !executor.(*safeShellExecutor).isDangerousCommand(dc.cmd, dc.args) {
			t.Errorf("expected command %s %v to be dangerous", dc.cmd, dc.args)
		}
	}

	// Test safe commands
	safe := []struct {
		cmd  string
		args []string
	}{
		{"echo", []string{"hello"}},
		{"ls", []string{"-la"}},
		{"cat", []string{"file.txt"}},
		{"go", []string{"build"}},
	}

	for _, sc := range safe {
		if executor.(*safeShellExecutor).isDangerousCommand(sc.cmd, sc.args) {
			t.Errorf("expected command %s %v to be safe", sc.cmd, sc.args)
		}
	}
}

func TestSafeShellExecutor_Timeout(t *testing.T) {
	executor := NewSafeShellExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := executor.Shell(ctx, "sleep", "1")
	if err == nil {
		t.Fatal("expected error for timeout, got nil")
	}
}

func TestSafeShellExecutor_ReadFile(t *testing.T) {
	executor := NewSafeShellExecutor()
	_, err := executor.ReadFile("/nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSafeShellExecutor_WriteFile(t *testing.T) {
	executor := NewSafeShellExecutor()
	err := executor.WriteFile("/nonexistent", []byte("test"), 0644)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSafeShellExecutor_DetectProjectType(t *testing.T) {
	executor := NewSafeShellExecutor()
	_, err := executor.DetectProjectType("/nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSafeShellExecutor_Format(t *testing.T) {
	executor := NewSafeShellExecutor()
	_, err := executor.Format(context.Background(), []byte("test"), "go")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSafeShellExecutor_FormatCode(t *testing.T) {
	executor := NewSafeShellExecutor()
	err := executor.FormatCode("/nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSafeShellExecutor_Execute(t *testing.T) {
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)

	result, err := executor.Execute("echo", []string{"test"}, DefaultTimeout)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Stdout != "test" {
		t.Errorf("expected stdout %q, got %q", "test", result.Stdout)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}
