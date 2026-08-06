package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// NewExecutor Tests
// ============================================================================

func TestNewExecutor_GoProject(t *testing.T) {
	info := &ProjectInfo{
		Type:      ProjectTypeGo,
		RootDir:   "/test/go",
		SrcDir:    ".",
		TestDir:   ".",
		BuildFile: "go.mod",
	}

	executor := NewExecutor(info)
	if executor == nil {
		t.Fatal("expected non-nil executor")
	}

	// Verify it's a goToolExecutor by testing WriteFile
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := executor.WriteFile(testFile, "test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestNewExecutor_JavaProject(t *testing.T) {
	info := &ProjectInfo{
		Type:      ProjectTypeJava,
		RootDir:   "/test/java",
		SrcDir:    "src/main/java",
		TestDir:   "src/test/java",
		BuildFile: "build.gradle",
	}

	executor := NewExecutor(info)
	if executor == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestNewExecutor_KotlinProject(t *testing.T) {
	info := &ProjectInfo{
		Type:      ProjectTypeKotlin,
		RootDir:   "/test/kotlin",
		SrcDir:    "src/main/kotlin",
		TestDir:   "src/test/kotlin",
		BuildFile: "build.gradle.kts",
	}

	executor := NewExecutor(info)
	if executor == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestNewExecutor_RustProject(t *testing.T) {
	info := &ProjectInfo{
		Type:      ProjectTypeRust,
		RootDir:   "/test/rust",
		SrcDir:    "src",
		TestDir:   "src",
		BuildFile: "Cargo.toml",
	}

	executor := NewExecutor(info)
	if executor == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestNewExecutor_TypeScriptProject(t *testing.T) {
	info := &ProjectInfo{
		Type:      ProjectTypeTypeScript,
		RootDir:   "/test/typescript",
		SrcDir:    "src",
		TestDir:   "src",
		BuildFile: "package.json",
	}

	executor := NewExecutor(info)
	if executor == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestNewExecutor_PythonProject(t *testing.T) {
	info := &ProjectInfo{
		Type:      ProjectTypePython,
		RootDir:   "/test/python",
		SrcDir:    ".",
		TestDir:   ".",
		BuildFile: "requirements.txt",
	}

	executor := NewExecutor(info)
	if executor == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestNewExecutor_UnknownProject(t *testing.T) {
	info := &ProjectInfo{
		Type:      ProjectTypeUnknown,
		RootDir:   "/test/unknown",
		SrcDir:    ".",
		TestDir:   ".",
		BuildFile: "",
	}

	executor := NewExecutor(info)
	if executor == nil {
		t.Fatal("expected non-nil executor")
	}
}

// ============================================================================
// Generic Tool Executor Tests
// ============================================================================

func TestGenericToolExecutor_WriteFile(t *testing.T) {
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := executor.WriteFile(testFile, "test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(content) != "test" {
		t.Errorf("expected content %q, got %q", "test", string(content))
	}
}

func TestGenericToolExecutor_ReadFile(t *testing.T) {
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	expectedContent := "test content"
	if err := os.WriteFile(testFile, []byte(expectedContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	content, err := executor.ReadFile(testFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, content)
	}
}

func TestGenericToolExecutor_AppendToFile(t *testing.T) {
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("Initial\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	err := executor.AppendToFile(testFile, "Appended\n")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	expected := "Initial\nAppended\n"
	if string(content) != expected {
		t.Errorf("expected content %q, got %q", expected, string(content))
	}
}

func TestGenericToolExecutor_Execute(t *testing.T) {
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)

	result, err := executor.Execute("echo", []string{"test"}, DefaultTimeout)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Stdout != "test" {
		t.Errorf("expected stdout %q, got %q", "test", result.Stdout)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestGenericToolExecutor_FormatCode(t *testing.T) {
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)

	err := executor.FormatCode("/nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "formatting not supported") {
		t.Errorf("expected 'formatting not supported' error, got: %v", err)
	}
}
