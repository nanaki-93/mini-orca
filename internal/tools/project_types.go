package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectType represents the type of programming project.
type ProjectType string

const (
	ProjectTypeGo          ProjectType = "go"
	ProjectTypeKotlin      ProjectType = "kotlin"
	ProjectTypeJava        ProjectType = "java"
	ProjectTypeRust        ProjectType = "rust"
	ProjectTypeTypeScript  ProjectType = "typescript"
	ProjectTypePython      ProjectType = "python"
	ProjectTypeUnknown     ProjectType = "unknown"
)

// ProjectInfo holds information about a detected project.
type ProjectInfo struct {
	Type        ProjectType
	RootDir     string
	SourceDir   string
	BuildFiles  []string
	TestPattern string
}

// DetectProjectType detects the type of project in the given directory.
func DetectProjectType(dir string) (ProjectType, error) {
	info, err := AnalyzeProject(dir)
	if err != nil {
		return ProjectTypeUnknown, err
	}
	return info.Type, nil
}

// AnalyzeProject performs a full analysis of the project structure.
func AnalyzeProject(dir string) (*ProjectInfo, error) {
	info := &ProjectInfo{
		RootDir: dir,
	}

	// Check for go.mod
	if fileExists(filepath.Join(dir, "go.mod")) {
		info.Type = ProjectTypeGo
		info.SourceDir = dir
		info.BuildFiles = []string{"go.mod", "go.sum"}
		info.TestPattern = "./..."
		return info, nil
	}

	// Check for Gradle (Kotlin/Java)
	if fileExists(filepath.Join(dir, "build.gradle")) ||
		fileExists(filepath.Join(dir, "build.gradle.kts")) ||
		fileExists(filepath.Join(dir, "settings.gradle")) ||
		fileExists(filepath.Join(dir, "settings.gradle.kts")) {
		if dirExists(filepath.Join(dir, "src/main/kotlin")) ||
			fileExists(filepath.Join(dir, "build.gradle.kts")) {
			info.Type = ProjectTypeKotlin
		} else {
			info.Type = ProjectTypeJava
		}
		info.SourceDir = filepath.Join(dir, "src/main")
		info.BuildFiles = []string{"build.gradle", "gradlew"}
		info.TestPattern = "test"
		return info, nil
	}

	// Check for Cargo.toml (Rust)
	if fileExists(filepath.Join(dir, "Cargo.toml")) {
		info.Type = ProjectTypeRust
		info.SourceDir = filepath.Join(dir, "src")
		info.BuildFiles = []string{"Cargo.toml", "Cargo.lock"}
		info.TestPattern = "./..."
		return info, nil
	}

	// Check for package.json with TypeScript
	if fileExists(filepath.Join(dir, "package.json")) {
		content, _ := os.ReadFile(filepath.Join(dir, "package.json"))
		if strings.Contains(string(content), "typescript") ||
			strings.Contains(string(content), "tsc") ||
			strings.Contains(string(content), "@types/") {
			info.Type = ProjectTypeTypeScript
		} else {
			// Could be JS/Node.js project
			info.Type = ProjectTypeTypeScript
		}
		info.SourceDir = dir
		info.BuildFiles = []string{"package.json", "tsconfig.json"}
		info.TestPattern = "jest"
		return info, nil
	}

	// Check for pyproject.toml or requirements.txt (Python)
	if fileExists(filepath.Join(dir, "pyproject.toml")) ||
		fileExists(filepath.Join(dir, "requirements.txt")) ||
		fileExists(filepath.Join(dir, "setup.py")) {
		info.Type = ProjectTypePython
		info.SourceDir = dir
		info.BuildFiles = []string{"pyproject.toml", "requirements.txt"}
		info.TestPattern = "."
		return info, nil
	}

	// Check for tsconfig.json
	if fileExists(filepath.Join(dir, "tsconfig.json")) {
		info.Type = ProjectTypeTypeScript
		info.SourceDir = dir
		info.BuildFiles = []string{"tsconfig.json"}
		info.TestPattern = "jest"
		return info, nil
	}

	return nil, fmt.Errorf("unknown project type in directory: %s", dir)
}

// IsSupported checks if the project type is known and supported.
func (pt ProjectType) IsSupported() bool {
	switch pt {
	case ProjectTypeGo, ProjectTypeKotlin, ProjectTypeJava,
		ProjectTypeRust, ProjectTypeTypeScript, ProjectTypePython:
		return true
	default:
		return false
	}
}

// String returns the string representation of the project type.
func (pt ProjectType) String() string {
	return string(pt)
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// dirExists checks if a directory exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
