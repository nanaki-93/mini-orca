package tools

// ProjectType represents the type of a project detected in a directory.
type ProjectType string

const (
	// ProjectTypeGo represents a Go project.
	ProjectTypeGo ProjectType = "go"
	// ProjectTypeKotlin represents a Kotlin project.
	ProjectTypeKotlin ProjectType = "kotlin"
	// ProjectTypeJava represents a Java project.
	ProjectTypeJava ProjectType = "java"
	// ProjectTypeRust represents a Rust project.
	ProjectTypeRust ProjectType = "rust"
	// ProjectTypeTypeScript represents a TypeScript project.
	ProjectTypeTypeScript ProjectType = "typescript"
	// ProjectTypePython represents a Python project.
	ProjectTypePython ProjectType = "python"
	// ProjectTypeUnknown represents an unknown or unsupported project type.
	ProjectTypeUnknown ProjectType = "unknown"
)

// ProjectInfo holds metadata about a detected project.
type ProjectInfo struct {
	// Type is the detected project type.
	Type ProjectType
	// RootDir is the root directory of the project.
	RootDir string
	// SrcDir is the source code directory.
	SrcDir string
	// TestDir is the test code directory.
	TestDir string
	// BuildFile is the build configuration file name.
	BuildFile string
}
