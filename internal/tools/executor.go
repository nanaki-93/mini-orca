package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Executor is the main tool executor that dispatches commands based on project type.
type Executor struct {
	projectType ProjectType
	baseDir     string
}

// NewExecutor creates a new executor for the given project.
func NewExecutor(baseDir string) (*Executor, error) {
	pt, err := DetectProjectType(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to detect project type: %w", err)
	}

	return &Executor{
		projectType: pt,
		baseDir:     baseDir,
	}, nil
}

// NewExecutorWithProjectType creates an executor with a known project type.
func NewExecutorWithProjectType(baseDir string, pt ProjectType) *Executor {
	return &Executor{
		projectType: pt,
		baseDir:     baseDir,
	}
}

// ProjectType returns the detected project type.
func (e *Executor) ProjectType() ProjectType {
	return e.projectType
}

// RunShellCommand executes a shell command in the project directory.
func (e *Executor) RunShellCommand(cmd string, args ...string) (string, error) {
	fullArgs := append([]string{cmd}, args...)
	command := exec.Command(fullArgs[0], fullArgs[1:]...)
	command.Dir = e.baseDir

	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command %q failed: %w\nOutput: %s", strings.Join(fullArgs, " "), err, string(output))
	}

	return string(output), nil
}

// RunBuild runs the build command for the detected project type.
func (e *Executor) RunBuild() (string, error) {
	switch e.projectType {
	case ProjectTypeGo:
		return e.RunShellCommand("go", "build", "./...")
	case ProjectTypeKotlin:
		return e.RunShellCommand("./gradlew", "build", "-x", "test")
	case ProjectTypeJava:
		return e.RunShellCommand("./gradlew", "build", "-x", "test")
	case ProjectTypeRust:
		return e.RunShellCommand("cargo", "build")
	case ProjectTypeTypeScript:
		return e.RunShellCommand("npx", "tsc", "--noEmit")
	case ProjectTypePython:
		return "", fmt.Errorf("Python projects use pytest for testing, not a build step")
	default:
		return "", fmt.Errorf("unknown project type: %s", e.projectType)
	}
}

// RunTest runs the test command for the detected project type.
func (e *Executor) RunTest(target ...string) (string, error) {
	switch e.projectType {
	case ProjectTypeGo:
		if len(target) > 0 {
			return e.RunShellCommand("go", "test", "-v", target[0])
		}
		return e.RunShellCommand("go", "test", "-v", "./...")
	case ProjectTypeKotlin:
		return e.RunShellCommand("./gradlew", "test")
	case ProjectTypeJava:
		return e.RunShellCommand("./gradlew", "test")
	case ProjectTypeRust:
		return e.RunShellCommand("cargo", "test")
	case ProjectTypeTypeScript:
		return e.RunShellCommand("npx", "jest")
	case ProjectTypePython:
		return e.RunShellCommand("pytest", "-v")
	default:
		return "", fmt.Errorf("unknown project type: %s", e.projectType)
	}
}

// RunFormat runs the formatter for the detected project type.
func (e *Executor) RunFormat(target ...string) (string, error) {
	switch e.projectType {
	case ProjectTypeGo:
		if len(target) > 0 {
			return e.RunShellCommand("go", "fmt", target[0])
		}
		return e.RunShellCommand("go", "fmt", "./...")
	case ProjectTypeKotlin:
		return e.RunShellCommand("./gradlew", "ktlintFormat")
	case ProjectTypeJava:
		return e.RunShellCommand("./gradlew", "spotlessApply")
	case ProjectTypeRust:
		return e.RunShellCommand("cargo", "fmt")
	case ProjectTypeTypeScript:
		return e.RunShellCommand("npx", "prettier", "--write", "--ignore-unknown")
	case ProjectTypePython:
		return e.RunShellCommand("black", ".")
	default:
		return "", fmt.Errorf("unknown project type: %s", e.projectType)
	}
}

// GetSupportedExtensions returns the file extensions supported by this project type.
func (e *Executor) GetSupportedExtensions() []string {
	switch e.projectType {
	case ProjectTypeGo:
		return []string{".go"}
	case ProjectTypeKotlin:
		return []string{".kt", ".kts"}
	case ProjectTypeJava:
		return []string{".java"}
	case ProjectTypeRust:
		return []string{".rs"}
	case ProjectTypeTypeScript:
		return []string{".ts", ".tsx", ".js", ".jsx"}
	case ProjectTypePython:
		return []string{".py"}
	default:
		return []string{}
	}
}

// FindSourceFiles returns all source files in the project.
func (e *Executor) FindSourceFiles() ([]string, error) {
	extensions := e.GetSupportedExtensions()
	var files []string

	err := filepath.Walk(e.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip common non-source directories
			if info.Name() == ".git" || info.Name() == "node_modules" ||
				info.Name() == "vendor" || info.Name() == ".gradle" ||
				info.Name() == "target" || info.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		for _, supported := range extensions {
			if ext == supported {
				rel, err := filepath.Rel(e.baseDir, path)
				if err != nil {
					return err
				}
				files = append(files, rel)
				break
			}
		}
		return nil
	})

	return files, err
}

// GetFileLanguage returns the language name for a file extension.
func (e *Executor) GetFileLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".go":
		return "go"
	case ".kt", ".kts":
		return "kotlin"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	default:
		return "unknown"
	}
}
