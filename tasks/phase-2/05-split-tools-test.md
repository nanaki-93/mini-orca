# Task 2.5: Split internal/tools/tools_test.go

## Goal
Split 1452-line test file into multiple focused test files.

## Files to CREATE
- `internal/tools/tools_test.go` (~100 lines) — Common test helpers, setup
- `internal/tools/file_ops_test.go` (~200 lines) — File operation tests
- `internal/tools/formatter_test.go` (~150 lines) — Formatter tests
- `internal/tools/executor_test.go` (~200 lines) — Generic executor tests
- `internal/tools/go_executor_test.go` — Already exists (~150 lines), keep as is
- `internal/tools/python_executor_test.go` (~150 lines) — Python executor tests
- `internal/tools/typescript_executor_test.go` (~150 lines) — TypeScript executor tests
- `internal/tools/rust_executor_test.go` (~150 lines) — Rust executor tests
- `internal/tools/java_executor_test.go` (~150 lines) — Java executor tests
- `internal/tools/kotlin_executor_test.go` (~150 lines) — Kotlin executor tests

## Files to DELETE
- `internal/tools/tools_test.go` (original)

---

## Implementation Steps

### Step 1: Create tools_test.go — Common helpers
```go
package tools

import (
    "os"
    "path/filepath"
    "testing"
)

// createTempDir creates a temporary directory for tests
func createTempDir(t *testing.T) string { ... }

// cleanupTempDir removes a temporary directory
func cleanupTempDir(t *testing.T, dir string) { ... }

// createTestFile creates a test file with given content
func createTestFile(t *testing.T, dir, name, content string) string { ... }

// readTestFile reads a test file and returns its content
func readTestFile(t *testing.T, path string) string { ... }

// assertEqual asserts two values are equal
func assertEqual(t *testing.T, expected, actual interface{}, msg ...string) { ... }

// assertError asserts an error is returned
func assertError(t *testing.T, err error, msg ...string) { ... }
```

### Step 2: Create file_ops_test.go
Move all file operation tests from tools_test.go:
- `TestWriteFile`
- `TestReadFile`
- `TestDeleteFile`
- `TestListDir`
- `TestCopyFile`
- `TestMoveFile`
- `TestCreateDir`
- `TestDirExists`

### Step 3: Create formatter_test.go
Move all formatter tests:
- `TestFormatGoCode`
- `TestFormatPythonCode`
- `TestFormatTypeScriptCode`
- `TestFormatUnsupported`

### Step 4: Create executor_test.go
Move generic executor tests:
- `TestNewExecutor`
- `TestExecuteCommand`
- `TestExecuteCommandError`
- `TestSetWorkingDir`

### Step 5: Move language-specific executor tests
Move each language executor test to its own file:
- Go executor tests → `go_executor_test.go` (already exists)
- Python executor tests → `python_executor_test.go`
- TypeScript executor tests → `typescript_executor_test.go`
- Rust executor tests → `rust_executor_test.go`
- Java executor tests → `java_executor_test.go`
- Kotlin executor tests → `kotlin_executor_test.go`

## Verification
- `go test ./internal/tools/...` passes
- No file exceeds 300 lines (tests can go up to 400)
- All tests from original file are accounted for
