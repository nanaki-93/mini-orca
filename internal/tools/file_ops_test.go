package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// File Operations Tests
// ============================================================================

func TestFileOpsExecutor_ReadFile(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.txt", "Hello, World!")

	executor := NewFileOpsExecutor()
	data, err := executor.ReadFile(testFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if string(data) != "Hello, World!" {
		t.Errorf("expected content %q, got %q", "Hello, World!", string(data))
	}
}

func TestFileOpsExecutor_ReadFile_NotFound(t *testing.T) {
	executor := NewFileOpsExecutor()
	_, err := executor.ReadFile("/nonexistent/path/file.txt")
	assertError(t, err, "expected error for nonexistent file")
}

func TestFileOpsExecutor_WriteFile(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("Test content")

	executor := NewFileOpsExecutor()
	err := executor.WriteFile(testFile, testContent, 0644)
	assertNoError(t, err)

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(data) != string(testContent) {
		t.Errorf("expected content %q, got %q", string(testContent), string(data))
	}
}

func TestFileOpsExecutor_WriteFile_Atomically(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Atomic write test"

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	err := executor.WriteFileAtomic(testFile, testContent)
	assertNoError(t, err)

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(data) != testContent {
		t.Errorf("expected content %q, got %q", testContent, string(data))
	}
}

func TestFileOpsExecutor_WriteFileAtomic_NestedDirectories(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := filepath.Join(tmpDir, "nested", "dir", "test.txt")
	testContent := "Nested directory test"

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	err := executor.WriteFileAtomic(testFile, testContent)
	assertNoError(t, err)

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(data) != testContent {
		t.Errorf("expected content %q, got %q", testContent, string(data))
	}
}

func TestFileOpsExecutor_AppendFunctionToFile(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.go", "package main\n\nimport \"fmt\"\n")

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	functionBody := "func Hello() {\n\tfmt.Println(\"Hello\")\n}"

	err := executor.AppendFunctionToFile(testFile, "Hello", functionBody)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content := readTestFile(t, testFile)
	if !strings.Contains(content, "func Hello()") {
		t.Error("expected function to be appended to file")
	}
}

func TestFileOpsExecutor_AppendFunctionToFile_Duplicate(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.go", "package main\n\nfunc Hello() {\n\tfmt.Println(\"Hello\")\n}")

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	functionBody := "func Hello() {\n\tfmt.Println(\"Hello\")\n}"

	err := executor.AppendFunctionToFile(testFile, "Hello", functionBody)
	if err == nil {
		t.Fatal("expected error for duplicate function, got nil")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected error to contain 'already exists', got: %v", err)
	}
}

func TestFileOpsExecutor_WriteStructToFile(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.go", "package main\n\nimport \"fmt\"\n")

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	structBody := "type User struct {\n\tID   string\n\tName string\n}"

	err := executor.WriteStructToFile(testFile, "User", structBody)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content := readTestFile(t, testFile)
	if !strings.Contains(content, "type User struct") {
		t.Error("expected struct to be written to file")
	}
}

func TestFileOpsExecutor_WriteStructToFile_Duplicate(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.go", "package main\n\ntype User struct {\n\tID   string\n\tName string\n}")

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	structBody := "type User struct {\n\tID   string\n\tName string\n}"

	err := executor.WriteStructToFile(testFile, "User", structBody)
	if err == nil {
		t.Fatal("expected error for duplicate struct, got nil")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected error to contain 'already exists', got: %v", err)
	}
}

func TestFileOpsExecutor_UpdateFile(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.go", "package main\n\nimport \"fmt\"\n")

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	updates := []FileUpdate{
		{
			Type:    "append_function",
			Name:    "Hello",
			Content: "func Hello() {\n\tfmt.Println(\"Hello\")\n}",
		},
		{
			Type:    "append_struct",
			Name:    "User",
			Content: "type User struct {\n\tID   string\n\tName string\n}",
		},
	}

	err := executor.UpdateFile(testFile, updates)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content := readTestFile(t, testFile)
	if !strings.Contains(content, "func Hello()") {
		t.Error("expected function to be appended")
	}
	if !strings.Contains(content, "type User struct") {
		t.Error("expected struct to be appended")
	}
}

func TestFileOpsExecutor_UpdateFile_ApplyUpdatesInOrder(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.txt", "Initial content\n")

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	updates := []FileUpdate{
		{
			Type:    "replace",
			Name:    "",
			Content: "Replaced content\n",
		},
		{
			Type:    "replace",
			Name:    "",
			Content: "Final content\n",
		},
	}

	err := executor.UpdateFile(testFile, updates)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content := readTestFile(t, testFile)
	if content != "Final content\n" {
		t.Errorf("expected final content, got: %q", content)
	}
}

func TestFileOpsExecutor_UpdateFile_UnknownUpdateType(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.txt", "test")

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	updates := []FileUpdate{
		{
			Type:    "unknown_type",
			Name:    "",
			Content: "test",
		},
	}

	err := executor.UpdateFile(testFile, updates)
	if err == nil {
		t.Fatal("expected error for unknown update type, got nil")
	}

	if !strings.Contains(err.Error(), "unknown update type") {
		t.Errorf("expected error to contain 'unknown update type', got: %v", err)
	}
}

func TestFileOpsExecutor_ReadFileAsString(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.txt", "String content")

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	content, err := executor.ReadFileAsString(testFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if content != "String content" {
		t.Errorf("expected content %q, got %q", "String content", content)
	}
}

func TestFileOpsExecutor_AppendToFile(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	testFile := createTestFile(t, tmpDir, "test.txt", "Initial\n")

	// Use genericToolExecutor which implements ToolExecutor
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)
	err := executor.AppendToFile(testFile, "Appended\n")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content := readTestFile(t, testFile)
	expected := "Initial\nAppended\n"
	if content != expected {
		t.Errorf("expected content %q, got %q", expected, content)
	}
}
