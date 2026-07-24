// Package tools provides project utilities for Mini-Orca.
//
// The tools package provides project detection, file operations,
// git integration, and code formatting capabilities.
//
// # Project Type Detection
//
// Mini-Orca auto-detects project types by looking for build files:
//
//	Project Type    | Detection Files
//	----------------|----------------------------------
//	Go              | go.mod
//	Kotlin          | build.gradle.kts, src/main/kotlin
//	Java            | build.gradle, src/main/java
//	Rust            | Cargo.toml
//	TypeScript      | package.json, tsconfig.json
//	Python          | pyproject.toml, requirements.txt
//
// # Tool Executor
//
// The Executor dispatches commands based on project type:
//   - RunShellCommand: Execute arbitrary shell commands
//   - RunBuildCommand: Run project-specific build commands
//   - RunTestCommand: Run project-specific test commands
//   - RunLintCommand: Run linter for the project type
//
// # File Operations
//
// FileOps provides safe file operations:
//   - ReadFile: Read file contents
//   - WriteFile: Atomic file writes (write to temp, then rename)
//   - ReadDirectory: List directory contents
//   - FindFiles: Find files matching a pattern
//
// # Git Operations
//
// GitOps provides git integration:
//   - Status: Get git status
//   - Add: Stage files
//   - Commit: Commit with auto-generated messages
//   - Diff: Get diff of staged/unstaged changes
//   - Init: Initialize git repository
//
// # Formatter
//
// Formatter provides code formatting:
//   - Format: Format code based on project type
//   - Supported: gofmt, ktlint, rustfmt, prettier, black
//
// # Example
//
//	executor, _ := tools.NewExecutor("/path/to/project")
//	output, _ := executor.RunBuildCommand()
//
//	gitOps := tools.NewGitOps("/path/to/project")
//	gitOps.Commit("feat: implement user authentication")
package tools
