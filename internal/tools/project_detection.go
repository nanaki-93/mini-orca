package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// projectDetectorExecutor implements project detection for the Executor interface.
type projectDetectorExecutor struct {
	formatter Executor
}

// NewProjectDetectorExecutor creates a new project detector executor instance.
func NewProjectDetectorExecutor() Executor {
	return &projectDetectorExecutor{
		formatter: NewFormatterExecutor(),
	}
}

// _ ensures NewFormatterExecutor is used.
var _ = NewFormatterExecutor

// DetectProjectType detects the type of project at the given directory.
func (p *projectDetectorExecutor) DetectProjectType(dirPath string) (*ProjectInfo, error) {
	info := &ProjectInfo{
		RootDir: dirPath,
	}

	// Check for Go project
	if p.hasGoMod(dirPath) {
		info.Type = ProjectTypeGo
		info.SrcDir = "."
		info.TestDir = "."
		info.BuildFile = "go.mod"
		return info, nil
	}

	// Check for Kotlin/Java project
	if p.hasGradleBuild(dirPath) {
		if p.isKotlinProject(dirPath) {
			info.Type = ProjectTypeKotlin
			info.SrcDir = "src/main/kotlin"
			info.TestDir = "src/test/kotlin"
		} else {
			info.Type = ProjectTypeJava
			info.SrcDir = "src/main/java"
			info.TestDir = "src/test/java"
		}
		if _, err := os.Stat(filepath.Join(dirPath, "build.gradle.kts")); err == nil {
			info.BuildFile = "build.gradle.kts"
		} else {
			info.BuildFile = "build.gradle"
		}
		return info, nil
	}

	// Check for Rust project
	if p.hasCargoToml(dirPath) {
		info.Type = ProjectTypeRust
		info.SrcDir = "src"
		info.TestDir = "src"
		info.BuildFile = "Cargo.toml"
		return info, nil
	}

	// Check for TypeScript project
	if p.hasPackageJSON(dirPath) {
		info.Type = ProjectTypeTypeScript
		info.SrcDir = "src"
		info.TestDir = "src"
		info.BuildFile = "package.json"
		return info, nil
	}

	// Check for Python project
	if p.hasRequirementsTxt(dirPath) || p.hasPyProject(dirPath) {
		info.Type = ProjectTypePython
		info.SrcDir = "."
		info.TestDir = "."
		if p.hasPyProject(dirPath) {
			info.BuildFile = "pyproject.toml"
		} else {
			info.BuildFile = "requirements.txt"
		}
		return info, nil
	}

	return nil, fmt.Errorf("project_detection: no known project type found in %s", dirPath)
}

func (p *projectDetectorExecutor) isKotlinProject(dirPath string) bool {
	if _, err := os.Stat(filepath.Join(dirPath, "src", "main", "kotlin")); err == nil {
		return true
	}
	for _, buildFile := range []string{"build.gradle.kts", "build.gradle"} {
		data, err := os.ReadFile(filepath.Join(dirPath, buildFile))
		if err == nil && (contains(data, "kotlin(") || contains(data, "org.jetbrains.kotlin")) {
			return true
		}
	}
	return false
}

func contains(data []byte, value string) bool {
	return strings.Contains(string(data), value)
}

// hasGoMod checks if a go.mod file exists in the directory.
func (p *projectDetectorExecutor) hasGoMod(dirPath string) bool {
	_, err := os.Stat(filepath.Join(dirPath, "go.mod"))
	return err == nil
}

// hasGradleBuild checks if a build.gradle or build.gradle.kts file exists.
func (p *projectDetectorExecutor) hasGradleBuild(dirPath string) bool {
	gradlePath := filepath.Join(dirPath, "build.gradle")
	gradleKtsPath := filepath.Join(dirPath, "build.gradle.kts")
	_, errGradle := os.Stat(gradlePath)
	_, errGradleKts := os.Stat(gradleKtsPath)
	return errGradle == nil || errGradleKts == nil
}

// hasCargoToml checks if a Cargo.toml file exists in the directory.
func (p *projectDetectorExecutor) hasCargoToml(dirPath string) bool {
	_, err := os.Stat(filepath.Join(dirPath, "Cargo.toml"))
	return err == nil
}

// hasPackageJSON checks if a package.json file exists in the directory.
func (p *projectDetectorExecutor) hasPackageJSON(dirPath string) bool {
	_, err := os.Stat(filepath.Join(dirPath, "package.json"))
	return err == nil
}

// hasRequirementsTxt checks if a requirements.txt file exists in the directory.
func (p *projectDetectorExecutor) hasRequirementsTxt(dirPath string) bool {
	_, err := os.Stat(filepath.Join(dirPath, "requirements.txt"))
	return err == nil
}

// hasPyProject checks if a pyproject.toml file exists in the directory.
func (p *projectDetectorExecutor) hasPyProject(dirPath string) bool {
	_, err := os.Stat(filepath.Join(dirPath, "pyproject.toml"))
	return err == nil
}

// Shell executes a shell command and returns its output.
func (p *projectDetectorExecutor) Shell(ctx context.Context, command string, args ...string) (*ShellResult, error) {
	return p.formatter.Shell(ctx, command, args...)
}

// ReadFile reads the content of a file at the given path.
func (p *projectDetectorExecutor) ReadFile(path string) ([]byte, error) {
	return p.formatter.ReadFile(path)
}

// WriteFile writes content to a file at the given path with the specified permissions.
func (p *projectDetectorExecutor) WriteFile(path string, data []byte, perm uint32) error {
	return p.formatter.WriteFile(path, data, perm)
}

// Format formats code content using the appropriate formatter.
func (p *projectDetectorExecutor) Format(ctx context.Context, content []byte, language string) ([]byte, error) {
	return p.formatter.Format(ctx, content, language)
}

// FormatCode formats the code file at the given path.
func (p *projectDetectorExecutor) FormatCode(path string) error {
	return p.formatter.FormatCode(path)
}
