package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/logging"
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

	// FormatCode formats the code file at the given path.
	FormatCode(path string) error
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

// NewExecutor creates a ToolExecutor for the given project type.
// It dispatches to the correct language-specific executor based on ProjectInfo.Type.
// For unknown project types, it returns a generic shell-only executor.
func NewExecutor(projectInfo *ProjectInfo) ToolExecutor {
	switch projectInfo.Type {
	case ProjectTypeGo:
		return newGoToolExecutor(projectInfo)
	case ProjectTypeKotlin:
		return newKotlinToolExecutor(projectInfo)
	case ProjectTypeJava:
		return newJavaToolExecutor(projectInfo)
	case ProjectTypeRust:
		return newRustToolExecutor(projectInfo)
	case ProjectTypeTypeScript:
		return newTypeScriptToolExecutor(projectInfo)
	case ProjectTypePython:
		return newPythonToolExecutor(projectInfo)
	default:
		return newGenericToolExecutor(projectInfo)
	}
}

// genericToolExecutor implements ToolExecutor with basic shell operations.
type genericToolExecutor struct {
	info  *ProjectInfo
	shell Executor
}

func newGenericToolExecutor(info *ProjectInfo) *genericToolExecutor {
	return &genericToolExecutor{
		info:  info,
		shell: NewShellExecutor(),
	}
}

func (g *genericToolExecutor) Execute(cmd string, args []string, timeout time.Duration) (*ExecResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	result, err := g.shell.Shell(ctx, cmd, args...)
	duration := time.Since(start)

	return &ExecResult{
		ExitCode: result.ExitCode,
		Stdout:   result.Output,
		Stderr:   "",
		Duration: duration,
	}, err
}

func (g *genericToolExecutor) FormatCode(path string) error {
	return fmt.Errorf("generic_executor: formatting not supported for project type %s", g.info.Type)
}

func (g *genericToolExecutor) WriteFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func (g *genericToolExecutor) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (g *genericToolExecutor) AppendToFile(path string, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	return err
}

// goToolExecutor implements ToolExecutor for Go projects.
type goToolExecutor struct {
	genericToolExecutor
	goExecutor *goExecutor
}

func newGoToolExecutor(info *ProjectInfo) *goToolExecutor {
	return &goToolExecutor{
		genericToolExecutor: *newGenericToolExecutor(info),
		goExecutor:          NewGoExecutor(),
	}
}

func (g *goToolExecutor) FormatCode(path string) error {
	return g.goExecutor.FormatCode()
}

// kotlinToolExecutor implements ToolExecutor for Kotlin projects.
type kotlinToolExecutor struct {
	genericToolExecutor
	kotlinExecutor *kotlinExecutor
}

func newKotlinToolExecutor(info *ProjectInfo) *kotlinToolExecutor {
	return &kotlinToolExecutor{
		genericToolExecutor: *newGenericToolExecutor(info),
		kotlinExecutor:      NewKotlinExecutor(),
	}
}

func (k *kotlinToolExecutor) FormatCode(path string) error {
	return k.kotlinExecutor.FormatCode()
}

// javaToolExecutor implements ToolExecutor for Java projects.
type javaToolExecutor struct {
	genericToolExecutor
	javaExecutor *javaExecutor
}

func newJavaToolExecutor(info *ProjectInfo) *javaToolExecutor {
	return &javaToolExecutor{
		genericToolExecutor: *newGenericToolExecutor(info),
		javaExecutor:        NewJavaExecutor(),
	}
}

func (j *javaToolExecutor) FormatCode(path string) error {
	return j.javaExecutor.FormatCode()
}

// rustToolExecutor implements ToolExecutor for Rust projects.
type rustToolExecutor struct {
	genericToolExecutor
	rustExecutor *rustExecutor
}

func newRustToolExecutor(info *ProjectInfo) *rustToolExecutor {
	return &rustToolExecutor{
		genericToolExecutor: *newGenericToolExecutor(info),
		rustExecutor:        NewRustExecutor(),
	}
}

func (r *rustToolExecutor) FormatCode(path string) error {
	return r.rustExecutor.FormatCode()
}

// typescriptToolExecutor implements ToolExecutor for TypeScript projects.
type typescriptToolExecutor struct {
	genericToolExecutor
	typescriptExecutor *typescriptExecutor
}

func newTypeScriptToolExecutor(info *ProjectInfo) *typescriptToolExecutor {
	return &typescriptToolExecutor{
		genericToolExecutor: *newGenericToolExecutor(info),
		typescriptExecutor:  NewTypeScriptExecutor(),
	}
}

func (t *typescriptToolExecutor) FormatCode(path string) error {
	return t.typescriptExecutor.FormatCode()
}

// pythonToolExecutor implements ToolExecutor for Python projects.
type pythonToolExecutor struct {
	genericToolExecutor
	pythonExecutor *pythonExecutor
}

func newPythonToolExecutor(info *ProjectInfo) *pythonToolExecutor {
	return &pythonToolExecutor{
		genericToolExecutor: *newGenericToolExecutor(info),
		pythonExecutor:      NewPythonExecutor(),
	}
}

func (p *pythonToolExecutor) FormatCode(path string) error {
	return p.pythonExecutor.FormatCode()
}

// InitToolExecutor detects the project type at the current directory and creates
// the appropriate ToolExecutor for the detected project type.
func InitToolExecutor() (*ProjectInfo, ToolExecutor) {
	currentDir, err := os.Getwd()
	if err != nil {
		logging.Warn("Failed to get current directory", "error", err)
		currentDir = "."
	}
	return InitToolExecutorWithPath(currentDir)
}

// InitToolExecutorWithPath detects the project type at the given path and creates
// the appropriate ToolExecutor for the detected project type.
func InitToolExecutorWithPath(path string) (*ProjectInfo, ToolExecutor) {
	// Detect project type starting from the provided path
	detector := NewProjectDetectorExecutor()

	projectInfo, err := detector.DetectProjectType(path)
	if err != nil {
		// If project type detection fails, try parent directories
		logging.Warn("Failed to detect project type", "dir", path, "error", err)
		projectInfo, err = detector.DetectProjectType(filepath.Dir(path))
		if err != nil {
			logging.Warn("Failed to detect project type in parent directory", "error", err)
			logging.Info("Using generic executor (shell-only)")
			return nil, NewExecutor(&ProjectInfo{Type: ProjectTypeUnknown, RootDir: path})
		}
	}

	logging.Info("Detected project type", "type", projectInfo.Type)

	// Create ToolExecutor based on detected project type
	executor := NewExecutor(projectInfo)
	return projectInfo, executor
}
