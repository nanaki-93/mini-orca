package tools

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
)

// formatterExecutor implements code formatting for the Executor interface.
type formatterExecutor struct {
	fileOps Executor
}

// NewFormatterExecutor creates a new formatter executor instance.
func NewFormatterExecutor() Executor {
	return &formatterExecutor{
		fileOps: NewFileOpsExecutor(),
	}
}

// _ ensures NewFileOpsExecutor is used.
var _ = NewFileOpsExecutor

// Format formats code content using the appropriate formatter based on language.
func (f *formatterExecutor) Format(ctx context.Context, content []byte, language string) ([]byte, error) {
	switch language {
	case "go":
		return f.formatGo(ctx, content)
	default:
		return content, nil
	}
}

// formatGo formats Go code using gofmt.
func (f *formatterExecutor) formatGo(ctx context.Context, content []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gofmt")
	cmd.Stdin = bytes.NewReader(content)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("formatter: gofmt failed: %w, stderr: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// Shell executes a shell command and returns its output.
func (f *formatterExecutor) Shell(ctx context.Context, command string, args ...string) (*ShellResult, error) {
	return f.fileOps.Shell(ctx, command, args...)
}

// ReadFile reads the content of a file at the given path.
func (f *formatterExecutor) ReadFile(path string) ([]byte, error) {
	return f.fileOps.ReadFile(path)
}

// WriteFile writes content to a file at the given path with the specified permissions.
func (f *formatterExecutor) WriteFile(path string, data []byte, perm uint32) error {
	return f.fileOps.WriteFile(path, data, perm)
}

// DetectProjectType detects the type of project at the given directory.
func (f *formatterExecutor) DetectProjectType(dirPath string) (*ProjectInfo, error) {
	return f.fileOps.DetectProjectType(dirPath)
}

// FormatCode formats the code file at the given path by detecting the project type and delegating to the appropriate formatter.
func (f *formatterExecutor) FormatCode(path string) error {
	// Detect the project root directory
	projectRoot, err := f.findProjectRoot(path)
	if err != nil {
		return fmt.Errorf("formatter: failed to detect project type: %w", err)
	}

	// Detect project type
	projectInfo, err := f.DetectProjectType(projectRoot)
	if err != nil {
		return fmt.Errorf("formatter: failed to detect project type: %w", err)
	}

	// Delegate to language-specific formatter
	switch projectInfo.Type {
	case ProjectTypeGo:
		return f.formatGoFile(path)
	case ProjectTypeKotlin:
		return f.formatKotlinFile(path)
	case ProjectTypeJava:
		return f.formatJavaFile(path)
	case ProjectTypeRust:
		return f.formatRustFile(path)
	case ProjectTypeTypeScript:
		return f.formatTypeScriptFile(path)
	case ProjectTypePython:
		return f.formatPythonFile(path)
	default:
		return fmt.Errorf("formatter: unsupported project type: %s", projectInfo.Type)
	}
}

// findProjectRoot finds the project root directory starting from the file path.
func (f *formatterExecutor) findProjectRoot(filePath string) (string, error) {
	dir := filepath.Dir(filePath)

	// Walk up the directory tree to find the project root
	for {
		// Check for project markers
		if _, err := exec.LookPath("go"); err == nil {
			if _, err := f.fileOps.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
				return dir, nil
			}
		}
		if _, err := f.fileOps.ReadFile(filepath.Join(dir, "build.gradle")); err == nil {
			return dir, nil
		}
		if _, err := f.fileOps.ReadFile(filepath.Join(dir, "build.gradle.kts")); err == nil {
			return dir, nil
		}
		if _, err := f.fileOps.ReadFile(filepath.Join(dir, "Cargo.toml")); err == nil {
			return dir, nil
		}
		if _, err := f.fileOps.ReadFile(filepath.Join(dir, "package.json")); err == nil {
			return dir, nil
		}
		if _, err := f.fileOps.ReadFile(filepath.Join(dir, "requirements.txt")); err == nil {
			return dir, nil
		}
		if _, err := f.fileOps.ReadFile(filepath.Join(dir, "pyproject.toml")); err == nil {
			return dir, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root of the filesystem
			return "", fmt.Errorf("formatter: no project root found for %s", filePath)
		}
		dir = parent
	}
}

// formatGoFile formats a Go file using gofmt.
func (f *formatterExecutor) formatGoFile(path string) error {
	if _, err := exec.LookPath("gofmt"); err != nil {
		log.Printf("formatter: gofmt not installed, skipping formatting for %s", path)
		return nil
	}

	cmd := exec.Command("gofmt", "-w", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("formatter: gofmt failed: %w", err)
	}
	return nil
}

// formatKotlinFile formats a Kotlin file using ktlint.
func (f *formatterExecutor) formatKotlinFile(path string) error {
	if _, err := exec.LookPath("ktlint"); err != nil {
		log.Printf("formatter: ktlint not installed, skipping formatting for %s", path)
		return nil
	}

	cmd := exec.Command("ktlint", "--format", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("formatter: ktlint failed: %w", err)
	}
	return nil
}

// formatJavaFile formats a Java file using google-java-format.
func (f *formatterExecutor) formatJavaFile(path string) error {
	if _, err := exec.LookPath("google-java-format"); err != nil {
		log.Printf("formatter: google-java-format not installed, skipping formatting for %s", path)
		return nil
	}

	cmd := exec.Command("google-java-format", "--replace", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("formatter: google-java-format failed: %w", err)
	}
	return nil
}

// formatRustFile formats a Rust file using rustfmt.
func (f *formatterExecutor) formatRustFile(path string) error {
	if _, err := exec.LookPath("rustfmt"); err != nil {
		log.Printf("formatter: rustfmt not installed, skipping formatting for %s", path)
		return nil
	}

	cmd := exec.Command("rustfmt", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("formatter: rustfmt failed: %w", err)
	}
	return nil
}

// formatTypeScriptFile formats a TypeScript file using prettier.
func (f *formatterExecutor) formatTypeScriptFile(path string) error {
	if _, err := exec.LookPath("npx"); err != nil {
		log.Printf("formatter: npx not installed, skipping formatting for %s", path)
		return nil
	}

	cmd := exec.Command("npx", "prettier", "--write", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("formatter: prettier failed: %w", err)
	}
	return nil
}

// formatPythonFile formats a Python file using black.
func (f *formatterExecutor) formatPythonFile(path string) error {
	if _, err := exec.LookPath("black"); err != nil {
		log.Printf("formatter: black not installed, skipping formatting for %s", path)
		return nil
	}

	cmd := exec.Command("black", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("formatter: black failed: %w", err)
	}
	return nil
}
