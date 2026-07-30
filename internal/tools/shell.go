package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/internal/logging"
)

// DefaultTimeout is the default timeout for command execution.
const DefaultTimeout = 60 * time.Second

// shellExecutor implements the Executor interface for shell command execution.
type shellExecutor struct{}

// NewShellExecutor creates a new shell executor instance.
func NewShellExecutor() Executor {
	return &shellExecutor{}
}

// safeShellExecutor wraps shellExecutor with safety features: allowlist, dangerous command blocking, and logging.
type safeShellExecutor struct {
	shell           *shellExecutor
	allowedCommands map[string]bool
	timeout         time.Duration
}

// NewSafeShellExecutor creates a new safe shell executor with default settings.
func NewSafeShellExecutor() Executor {
	return &safeShellExecutor{
		shell: &shellExecutor{},
		allowedCommands: map[string]bool{
			"go": true, "gofmt": true, "golangci-lint": true, "go vet": true,
			"gradlew": true, "./gradlew": true,
			"cargo": true, "cargo fmt": true, "cargo test": true, "cargo build": true, "cargo clippy": true,
			"npx": true, "npx prettier": true, "npx jest": true, "npx tsc": true, "npx eslint": true,
			"black": true, "pytest": true, "flake8": true, "pylint": true,
			"python": true, "python3": true, "python -m": true, "python3 -m": true,
			"bash": true, "sh": true,
			"echo": true, "ls": true, "cat": true, "pwd": true, "which": true,
			"git": true, "git status": true, "git diff": true, "git log": true, "git add": true, "git commit": true, "git push": true, "git pull": true, "git merge": true,
			"make": true, "cmake": true,
			"mkdir": true, "touch": true, "cp": true, "mv": true,
			"tee": true, "head": true, "tail": true, "wc": true, "grep": true, "sed": true, "awk": true,
			"date": true, "whoami": true, "hostname": true,
		},
		timeout: DefaultTimeout,
	}
}

// isDangerousCommand checks if a command is dangerous and should be blocked.
func (s *safeShellExecutor) isDangerousCommand(cmd string, args []string) bool {
	dangerousPatterns := []string{
		"rm -rf", "rm -r", "rm -f", "rm --no-preserve-root",
		"mkfs", "mkfs.ext", "mkfs.ntfs", "mkfs.fat",
		"dd if=", "dd of=/",
		":(){ :|:& };:", "fork bomb",
		"> /dev/sda", "> /dev/sdb", "> /dev/hda",
		"chmod 777",
		"wget -O /etc", "curl -o /etc",
		"/etc/shadow", "/etc/passwd",
		"drop database", "truncate tables",
		"format ", "wipe",
	}

	fullCmd := cmd + " " + strings.Join(args, " ")
	for _, pattern := range dangerousPatterns {
		if strings.Contains(strings.ToLower(fullCmd), strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// isCommandAllowed checks if a command is in the allowlist.
func (s *safeShellExecutor) isCommandAllowed(cmd string) bool {
	// Check exact match
	if s.allowedCommands[cmd] {
		return true
	}

	// Check prefix match for commands with subcommands (e.g., "go build" matches "go")
	for allowedCmd := range s.allowedCommands {
		if strings.HasPrefix(cmd, allowedCmd) {
			return true
		}
	}
	return false
}

// logCommand logs the executed command for audit trail.
func (s *safeShellExecutor) logCommand(cmd string, args []string) {
	logging.Info("Executing command", "provider", "safe-shell", "cmd", cmd, "args", args)
}

// Shell executes a shell command with the given arguments and returns the result.
func (s *shellExecutor) Shell(ctx context.Context, command string, args ...string) (*ShellResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return nil, fmt.Errorf("shell: command execution failed: %w", err)
		}
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = strings.TrimSpace(stderr.String())
	}

	return &ShellResult{
		Output:   output,
		ExitCode: exitCode,
		Err:      err,
	}, nil
}

// ReadFile reads the content of a file at the given path.
func (s *shellExecutor) ReadFile(path string) ([]byte, error) {
	return nil, fmt.Errorf("shell: ReadFile not implemented")
}

// WriteFile writes content to a file at the given path.
func (s *shellExecutor) WriteFile(path string, data []byte, perm uint32) error {
	return fmt.Errorf("shell: WriteFile not implemented")
}

// DetectProjectType detects the type of project at the given directory.
func (s *shellExecutor) DetectProjectType(dirPath string) (*ProjectInfo, error) {
	return nil, fmt.Errorf("shell: DetectProjectType not implemented")
}

// Format formats code content using the appropriate formatter.
func (s *shellExecutor) Format(ctx context.Context, content []byte, language string) ([]byte, error) {
	return nil, fmt.Errorf("shell: Format not implemented")
}

// FormatCode formats the code file at the given path.
func (s *shellExecutor) FormatCode(path string) error {
	return fmt.Errorf("shell: FormatCode not implemented")
}

// Execute runs a command with the given arguments and timeout, returning a structured result.
func (s *shellExecutor) Execute(cmd string, args []string, timeout time.Duration) (*ExecResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	result, err := s.Shell(ctx, cmd, args...)
	duration := time.Since(start)

	return &ExecResult{
		ExitCode: result.ExitCode,
		Stdout:   result.Output,
		Stderr:   "",
		Duration: duration,
	}, err
}

// Shell executes a shell command with the given arguments and returns the result.
func (s *safeShellExecutor) Shell(ctx context.Context, command string, args ...string) (*ShellResult, error) {
	// Check if command is allowed
	if !s.isCommandAllowed(command) {
		return nil, fmt.Errorf("safe-shell: command not allowed: %s", command)
	}

	// Check for dangerous commands
	if s.isDangerousCommand(command, args) {
		logging.Warn("Blocked dangerous command", "provider", "safe-shell", "cmd", command, "args", args)
		return nil, fmt.Errorf("safe-shell: dangerous command blocked: %s %s", command, strings.Join(args, " "))
	}

	// Log the command
	s.logCommand(command, args)

	// Execute with timeout
	timeout := s.timeout
	if ctx.Err() != nil {
		timeout = 0
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return s.shell.Shell(cmdCtx, command, args...)
}

// ReadFile reads the content of a file at the given path.
func (s *safeShellExecutor) ReadFile(path string) ([]byte, error) {
	return s.shell.ReadFile(path)
}

// WriteFile writes content to a file at the given path.
func (s *safeShellExecutor) WriteFile(path string, data []byte, perm uint32) error {
	return s.shell.WriteFile(path, data, perm)
}

// DetectProjectType detects the type of project at the given directory.
func (s *safeShellExecutor) DetectProjectType(dirPath string) (*ProjectInfo, error) {
	return s.shell.DetectProjectType(dirPath)
}

// Format formats code content using the appropriate formatter.
func (s *safeShellExecutor) Format(ctx context.Context, content []byte, language string) ([]byte, error) {
	return s.shell.Format(ctx, content, language)
}

// FormatCode formats the code file at the given path.
func (s *safeShellExecutor) FormatCode(path string) error {
	return s.shell.FormatCode(path)
}

// Execute runs a command with the given arguments and timeout, returning a structured result.
func (s *safeShellExecutor) Execute(cmd string, args []string, timeout time.Duration) (*ExecResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	result, err := s.Shell(ctx, cmd, args...)
	duration := time.Since(start)

	return &ExecResult{
		ExitCode: result.ExitCode,
		Stdout:   result.Output,
		Stderr:   "",
		Duration: duration,
	}, err
}
