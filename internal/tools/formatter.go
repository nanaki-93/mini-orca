package tools

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Formatter provides code formatting for different languages.
type Formatter struct {
	toolExecutor *Executor
}

// NewFormatter creates a new formatter.
func NewFormatter(toolExecutor *Executor) *Formatter {
	return &Formatter{
		toolExecutor: toolExecutor,
	}
}

// FormatFile formats a single file.
func (f *Formatter) FormatFile(relPath string) error {
	fullPath := filepath.Join(f.toolExecutor.baseDir, relPath)

	switch f.toolExecutor.projectType {
	case ProjectTypeGo:
		cmd := exec.Command("gofmt", "-w", fullPath)
		return cmd.Run()

	case ProjectTypeKotlin:
		// Try ktlint
		ktlintPath, err := f.findExecutable("ktlint")
		if err == nil {
			cmd := exec.Command(ktlintPath, "--format", fullPath)
			return cmd.Run()
		}
		return fmt.Errorf("ktlint not found, skipping Kotlin formatting")

	case ProjectTypeJava:
		// Java formatting via gradle
		_, err := f.toolExecutor.RunShellCommand("./gradlew", "spotlessApply")
		return err

	case ProjectTypeRust:
		cmd := exec.Command("rustfmt", fullPath)
		return cmd.Run()

	case ProjectTypeTypeScript:
		// Try prettier
		prettierPath, err := f.findExecutable("npx")
		if err == nil {
			cmd := exec.Command(prettierPath, "prettier", "--write", fullPath)
			cmd.Dir = f.toolExecutor.baseDir
			return cmd.Run()
		}
		return fmt.Errorf("prettier not found, skipping TypeScript formatting")

	case ProjectTypePython:
		// Try black
		blackPath, err := f.findExecutable("black")
		if err == nil {
			cmd := exec.Command(blackPath, fullPath)
			return cmd.Run()
		}
		return fmt.Errorf("black not found, skipping Python formatting")

	default:
		return fmt.Errorf("no formatter available for project type: %s", f.toolExecutor.projectType)
	}
}

// FormatAll formats all source files in the project.
func (f *Formatter) FormatAll() error {
	files, err := f.toolExecutor.FindSourceFiles()
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := f.FormatFile(file); err != nil {
			// Log but continue with other files
			fmt.Printf("Warning: failed to format %s: %v\n", file, err)
		}
	}

	return nil
}

// FormatAfterCode generates and formats new code for a single atomic unit.
func (f *Formatter) FormatAfterCode(relPath string, code string) error {
	// Write the code first
	fileOps := NewFileOps(f.toolExecutor.baseDir)

	// Read existing content
	existing, err := fileOps.ReadFile(relPath)
	if err != nil {
		// File doesn't exist, write new
		return fileOps.WriteFile(relPath, code+"\n")
	}

	// Append code
	newContent := strings.TrimRight(existing, "\n") + "\n\n" + code + "\n"

	if err := fileOps.WriteFile(relPath, newContent); err != nil {
		return err
	}

	// Format the file
	return f.FormatFile(relPath)
}

// findExecutable finds the path to an executable.
func (f *Formatter) findExecutable(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return path, nil
}
