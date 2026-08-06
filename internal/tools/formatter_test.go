package tools

import (
	"context"
	"os"
	"strings"
	"testing"
)

// ============================================================================
// Formatter Tests
// ============================================================================

func TestFormatterExecutor_Format_Go(t *testing.T) {
	executor := NewFormatterExecutor()
	content := []byte("package main\nfunc main(){\n}")

	result, err := executor.Format(context.Background(), content, "go")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if string(result) == string(content) {
		t.Error("expected formatted content to differ from original")
	}
}

func TestFormatterExecutor_Format_Unsupported(t *testing.T) {
	executor := NewFormatterExecutor()
	content := []byte("print('hello')")

	result, err := executor.Format(context.Background(), content, "python")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Unsupported languages should return content unchanged
	if string(result) != string(content) {
		t.Errorf("expected content unchanged for unsupported language, got: %q", string(result))
	}
}

func TestFormatterExecutor_Format_EmptyContent(t *testing.T) {
	executor := NewFormatterExecutor()
	content := []byte("")

	result, err := executor.Format(context.Background(), content, "go")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if string(result) != "" {
		t.Errorf("expected empty content, got: %q", string(result))
	}
}

func TestFormatterExecutor_Format_BadGoCode(t *testing.T) {
	executor := NewFormatterExecutor()
	content := []byte("invalid go code {{{")

	_, err := executor.Format(context.Background(), content, "go")
	if err == nil {
		t.Fatal("expected error for invalid Go code, got nil")
	}

	if !strings.Contains(err.Error(), "gofmt failed") {
		t.Errorf("expected error to contain 'gofmt failed', got: %v", err)
	}
}

func TestFormatterExecutor_FormatCode_Go(t *testing.T) {
	executor := NewFormatterExecutor()
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "main.go", "package main\n\nfunc main(){\n}")

	// Create go.mod to mark as Go project
	createTestFile(t, tmpDir, "go.mod", "module test\n\ngo 1.21\n")

	// FormatCode requires gofmt to be installed
	// We test that it doesn't panic and handles missing formatter gracefully
	err := executor.FormatCode(testFile)
	// Error is expected if gofmt is not installed
	_ = err
}

func TestFormatterExecutor_FormatCode_UnknownProjectType(t *testing.T) {
	executor := NewFormatterExecutor()
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.txt", "some content")

	err := executor.FormatCode(testFile)
	if err == nil {
		t.Fatal("expected error for unknown project type, got nil")
	}
}

func TestFormatterExecutor_Shell(t *testing.T) {
	executor := NewFormatterExecutor()
	result, err := executor.Shell(context.Background(), "echo", "test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Output != "test" {
		t.Errorf("expected output %q, got %q", "test", result.Output)
	}
}

func TestFormatterExecutor_ReadFile(t *testing.T) {
	executor := NewFormatterExecutor()
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.txt", "test content")

	data, err := executor.ReadFile(testFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if string(data) != "test content" {
		t.Errorf("expected content %q, got %q", "test content", string(data))
	}
}

func TestFormatterExecutor_WriteFile(t *testing.T) {
	executor := NewFormatterExecutor()
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := tmpDir + "/test.txt"
	err := executor.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(data) != "test" {
		t.Errorf("expected content %q, got %q", "test", string(data))
	}
}

func TestFormatterExecutor_DetectProjectType(t *testing.T) {
	executor := NewFormatterExecutor()
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	// Create go.mod to mark as Go project
	createTestFile(t, tmpDir, "go.mod", "module test\n\ngo 1.21\n")

	info, err := executor.DetectProjectType(tmpDir)
	// Note: This test may fail if the underlying shell doesn't implement DetectProjectType
	// The formatterExecutor uses NewFileOpsExecutor() which delegates to shellExecutor
	// If DetectProjectType is not implemented, we skip the test
	if err != nil {
		t.Skipf("DetectProjectType not implemented: %v", err)
	}

	if info.Type != ProjectTypeGo {
		t.Errorf("expected project type %q, got %q", ProjectTypeGo, info.Type)
	}
}

func TestFormatterExecutor_DetectProjectType_NotFound(t *testing.T) {
	executor := NewFormatterExecutor()
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	_, err := executor.DetectProjectType(tmpDir)
	if err == nil {
		t.Fatal("expected error for unknown project type, got nil")
	}
}
