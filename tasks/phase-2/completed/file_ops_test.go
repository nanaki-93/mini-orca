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
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	executor := NewFileOpsExecutor()
	data, err := executor.ReadFile(testFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if string(data) != testContent {
		t.Errorf("expected content %q, got %q", testContent, string(data))
	}
}

func TestFileOpsExecutor_ReadFile_NotFound(t *testing.T) {
	executor := NewFileOpsExecutor()
	_, err := executor.ReadFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestFileOpsExecutor_WriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("Test content")

	executor := NewFileOpsExecutor()
	err := executor.WriteFile(testFile, testContent, 0644)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(data) != string(testContent) {
		t.Errorf("expected content %q, got %q", string(testContent), string(data))
	}
}

func TestFileOpsExecutor_WriteFile_Atomically(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Atomic write test"

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	err := executor.WriteFileAtomic(testFile, testContent)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(data) != testContent {
		t.Errorf("expected content %q, got %q", testContent, string(data))
	}
}

func TestFileOpsExecutor_WriteFileAtomic_NestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "nested", "dir", "test.txt")
	testContent := "Nested directory test"

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	err := executor.WriteFileAtomic(testFile, testContent)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(data) != testContent {
		t.Errorf("expected content %q, got %q", testContent, string(data))
	}
}

func TestFileOpsExecutor_AppendFunctionToFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	// Create initial file content
	initialContent := "package main\n\nimport \"fmt\"\n"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	functionBody := "func Hello() {\n\tfmt.Println(\"Hello\")\n}"

	err := executor.AppendFunctionToFile(testFile, "Hello", functionBody)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !strings.Contains(string(content), "func Hello()") {
		t.Error("expected function to be appended to file")
	}
}

func TestFileOpsExecutor_AppendFunctionToFile_Duplicate(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	// Create file with existing function
	existingContent := "package main\n\nfunc Hello() {\n\tfmt.Println(\"Hello\")\n}"
	if err := os.WriteFile(testFile, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

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
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	// Create initial file content
	initialContent := "package main\n\nimport \"fmt\"\n"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	structBody := "type User struct {\n\tID   string\n\tName string\n}"

	err := executor.WriteStructToFile(testFile, "User", structBody)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !strings.Contains(string(content), "type User struct") {
		t.Error("expected struct to be written to file")
	}
}

func TestFileOpsExecutor_WriteStructToFile_Duplicate(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	// Create file with existing struct
	existingContent := "package main\n\ntype User struct {\n\tID   string\n\tName string\n}"
	if err := os.WriteFile(testFile, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

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
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	// Create initial file content
	initialContent := "package main\n\nimport \"fmt\"\n"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

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

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !strings.Contains(string(content), "func Hello()") {
		t.Error("expected function to be appended")
	}
	if !strings.Contains(string(content), "type User struct") {
		t.Error("expected struct to be appended")
	}
}

func TestFileOpsExecutor_UpdateFile_ApplyUpdatesInOrder(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create initial file content
	initialContent := "Initial content\n"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

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

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(content) != "Final content\n" {
		t.Errorf("expected final content, got: %q", string(content))
	}
}

func TestFileOpsExecutor_UpdateFile_UnknownUpdateType(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

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
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "String content"

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	executor := NewFileOpsExecutor().(*fileOpsExecutor)
	content, err := executor.ReadFileAsString(testFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if content != testContent {
		t.Errorf("expected content %q, got %q", testContent, content)
	}
}

func TestFileOpsExecutor_AppendToFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create initial file
	if err := os.WriteFile(testFile, []byte("Initial\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Use genericToolExecutor which implements ToolExecutor
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)
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

// ============================================================================
// Helper Function Tests (from file_ops.go)
// ============================================================================

func TestContainsFunction(t *testing.T) {
	executor := NewFileOpsExecutor()

	tests := []struct {
		name         string
		content      string
		functionName string
		shouldExist  bool
	}{
		{
			name:         "Go function",
			content:      "package main\n\nfunc Hello() {\n}",
			functionName: "Hello",
			shouldExist:  true,
		},
		{
			name:         "Go function with space",
			content:      "package main\n\nfunc Hello {\n}",
			functionName: "Hello",
			shouldExist:  true,
		},
		{
			name:         "Python function",
			content:      "def Hello():\n    pass",
			functionName: "Hello",
			shouldExist:  true,
		},
		{
			name:         "Java method",
			content:      "public Hello() {}",
			functionName: "Hello",
			shouldExist:  true,
		},
		{
			name:         "Non-existent function",
			content:      "package main\n\nfunc World() {\n}",
			functionName: "Hello",
			shouldExist:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.(*fileOpsExecutor).containsFunction(tt.content, tt.functionName)
			if result != tt.shouldExist {
				t.Errorf("expected %v, got %v for function %q in content %q", tt.shouldExist, result, tt.functionName, tt.content)
			}
		})
	}
}

func TestContainsStruct(t *testing.T) {
	executor := NewFileOpsExecutor()

	tests := []struct {
		name        string
		content     string
		structName  string
		shouldExist bool
	}{
		{
			name:        "Go struct",
			content:     "package main\n\ntype User struct {\n\tID string\n}",
			structName:  "User",
			shouldExist: true,
		},
		{
			name:        "Go struct with space",
			content:     "package main\n\ntype User {\n\tID string\n}",
			structName:  "User",
			shouldExist: true,
		},
		{
			name:        "Java class",
			content:     "public class User { private String ID; }",
			structName:  "User",
			shouldExist: true,
		},
		{
			name:        "Non-existent struct",
			content:     "package main\n\ntype Admin struct {\n\tID string\n}",
			structName:  "User",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.(*fileOpsExecutor).containsStruct(tt.content, tt.structName)
			if result != tt.shouldExist {
				t.Errorf("expected %v, got %v for struct %q in content %q", tt.shouldExist, result, tt.structName, tt.content)
			}
		})
	}
}

func TestContainsClass(t *testing.T) {
	executor := NewFileOpsExecutor()

	tests := []struct {
		name        string
		content     string
		className   string
		shouldExist bool
	}{
		{
			name:        "Java class",
			content:     "public class User { private String ID; }",
			className:   "User",
			shouldExist: true,
		},
		{
			name:        "Java class with extends",
			content:     "public class User extends Person { }",
			className:   "User",
			shouldExist: true,
		},
		{
			name:        "Java class with implements",
			content:     "public class User implements Cloneable { }",
			className:   "User",
			shouldExist: true,
		},
		{
			name:        "Interface",
			content:     "public interface User { }",
			className:   "User",
			shouldExist: true,
		},
		{
			name:        "Non-existent class",
			content:     "public class Admin { private String ID; }",
			className:   "User",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.(*fileOpsExecutor).containsClass(tt.content, tt.className)
			if result != tt.shouldExist {
				t.Errorf("expected %v, got %v for class %q in content %q", tt.shouldExist, result, tt.className, tt.content)
			}
		})
	}
}

func TestInsertStruct_GoFile(t *testing.T) {
	executor := NewFileOpsExecutor()

	existingContent := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}"
	structBody := "type User struct {\n\tID   string\n\tName string\n}"

	result := executor.(*fileOpsExecutor).insertStruct(existingContent, structBody)

	if !strings.Contains(result, "type User struct") {
		t.Error("expected struct to be inserted")
	}

	// Struct should be after imports
	importIdx := strings.Index(result, "import")
	structIdx := strings.Index(result, "type User struct")
	if importIdx == -1 || structIdx == -1 || importIdx > structIdx {
		t.Error("expected struct to be after imports")
	}
}

func TestInsertStruct_PythonFile(t *testing.T) {
	executor := NewFileOpsExecutor()

	existingContent := "import os\nimport sys\n\ndef main():\n    pass"
	structBody := "class User:\n    def __init__(self, id, name):\n        self.id = id\n        self.name = name"

	result := executor.(*fileOpsExecutor).insertStruct(existingContent, structBody)

	if !strings.Contains(result, "class User:") {
		t.Error("expected class to be inserted")
	}
}

func TestInsertStruct_EmptyContent(t *testing.T) {
	executor := NewFileOpsExecutor()

	structBody := "type User struct {\n\tID   string\n}"
	result := executor.(*fileOpsExecutor).insertStruct("", structBody)

	if result != structBody {
		t.Errorf("expected %q, got %q", structBody, result)
	}
}

func TestInsertClass_JavaFile(t *testing.T) {
	executor := NewFileOpsExecutor()

	existingContent := "package com.example;\n\nimport java.util.List;\n\npublic class Main {\n}"
	classBody := "public class User {\n    private String id;\n}"

	result := executor.(*fileOpsExecutor).insertClass(existingContent, classBody)

	if !strings.Contains(result, "public class User") {
		t.Error("expected class to be inserted")
	}
}

func TestInsertClass_EmptyContent(t *testing.T) {
	executor := NewFileOpsExecutor()

	classBody := "public class User {\n    private String id;\n}"
	result := executor.(*fileOpsExecutor).insertClass("", classBody)

	if result != classBody {
		t.Errorf("expected %q, got %q", classBody, result)
	}
}

func TestApplyUpdate_AppendFunction(t *testing.T) {
	executor := NewFileOpsExecutor()

	content := "package main\n"
	update := FileUpdate{
		Type:    "append_function",
		Name:    "Hello",
		Content: "func Hello() {\n}",
	}

	result, err := executor.(*fileOpsExecutor).applyUpdate(content, update)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(result, "func Hello()") {
		t.Error("expected function to be appended")
	}
}

func TestApplyUpdate_AppendStruct(t *testing.T) {
	executor := NewFileOpsExecutor()

	content := "package main\n"
	update := FileUpdate{
		Type:    "append_struct",
		Name:    "User",
		Content: "type User struct {\n\tID string\n}",
	}

	result, err := executor.(*fileOpsExecutor).applyUpdate(content, update)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(result, "type User struct") {
		t.Error("expected struct to be appended")
	}
}

func TestApplyUpdate_AppendClass(t *testing.T) {
	executor := NewFileOpsExecutor()

	content := "package main\n"
	update := FileUpdate{
		Type:    "append_class",
		Name:    "User",
		Content: "public class User {\n}",
	}

	result, err := executor.(*fileOpsExecutor).applyUpdate(content, update)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(result, "public class User") {
		t.Error("expected class to be appended")
	}
}

func TestApplyUpdate_Replace(t *testing.T) {
	executor := NewFileOpsExecutor()

	content := "old content"
	update := FileUpdate{
		Type:    "replace",
		Name:    "",
		Content: "new content",
	}

	result, err := executor.(*fileOpsExecutor).applyUpdate(content, update)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result != "new content" {
		t.Errorf("expected %q, got %q", "new content", result)
	}
}

func TestApplyUpdate_UnknownType(t *testing.T) {
	executor := NewFileOpsExecutor()

	content := "old content"
	update := FileUpdate{
		Type:    "unknown",
		Name:    "",
		Content: "new content",
	}

	_, err := executor.(*fileOpsExecutor).applyUpdate(content, update)
	if err == nil {
		t.Fatal("expected error for unknown update type, got nil")
	}

	if !strings.Contains(err.Error(), "unknown update type") {
		t.Errorf("expected 'unknown update type' error, got: %v", err)
	}
}
