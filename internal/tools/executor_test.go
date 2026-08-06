package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Generic Executor Tests
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
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)
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

func TestGenericToolExecutor_WriteFile(t *testing.T) {
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)

	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	err := executor.WriteFile(testFile, "test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content := readTestFile(t, testFile)
	if content != "test" {
		t.Errorf("expected content %q, got %q", "test", content)
	}
}

func TestGenericToolExecutor_ReadFile(t *testing.T) {
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)

	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)
	testFile := createTestFile(t, tmpDir, "test.txt", "test content")

	content, err := executor.ReadFile(testFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if content != "test content" {
		t.Errorf("expected content %q, got %q", "test content", content)
	}
}

func TestGenericToolExecutor_AppendToFile(t *testing.T) {
	info := &ProjectInfo{Type: ProjectTypeUnknown}
	executor := newGenericToolExecutor(info)

	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)
	testFile := createTestFile(t, tmpDir, "test.txt", "Initial\n")

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

	if result.Duration == 0 {
		t.Error("expected duration to be set")
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

// ============================================================================
// Project Detection Tests
// ============================================================================

func TestDetectProjectType_GoProject(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	// Create go.mod file
	createTestFile(t, tmpDir, "go.mod", "module example.com/test\n\ngo 1.21\n")

	detector := NewProjectDetectorExecutor()
	info, err := detector.DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if info.Type != ProjectTypeGo {
		t.Errorf("expected project type %q, got %q", ProjectTypeGo, info.Type)
	}
	if info.RootDir != tmpDir {
		t.Errorf("expected root dir %q, got %q", tmpDir, info.RootDir)
	}
	if info.BuildFile != "go.mod" {
		t.Errorf("expected build file %q, got %q", "go.mod", info.BuildFile)
	}
}

func TestDetectProjectType_JavaProject(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	// Create build.gradle file
	createTestFile(t, tmpDir, "build.gradle", "plugins { id 'java' }\n")

	detector := NewProjectDetectorExecutor()
	info, err := detector.DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if info.Type != ProjectTypeJava {
		t.Errorf("expected project type %q, got %q", ProjectTypeJava, info.Type)
	}
	if info.BuildFile != "build.gradle" {
		t.Errorf("expected build file %q, got %q", "build.gradle", info.BuildFile)
	}
}

func TestDetectProjectType_KotlinProject(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	// Create build.gradle.kts file
	createTestFile(t, tmpDir, "build.gradle.kts", "plugins { id(\"org.jetbrains.kotlin.jvm\") version \"1.9.0\" }\n")

	detector := NewProjectDetectorExecutor()
	info, err := detector.DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if info.Type != ProjectTypeJava {
		t.Errorf("expected project type %q, got %q", ProjectTypeJava, info.Type)
	}
}

func TestDetectProjectType_RustProject(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	// Create Cargo.toml file
	createTestFile(t, tmpDir, "Cargo.toml", "[package]\nname = \"test\"\nversion = \"0.1.0\"\n")

	detector := NewProjectDetectorExecutor()
	info, err := detector.DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if info.Type != ProjectTypeRust {
		t.Errorf("expected project type %q, got %q", ProjectTypeRust, info.Type)
	}
	if info.BuildFile != "Cargo.toml" {
		t.Errorf("expected build file %q, got %q", "Cargo.toml", info.BuildFile)
	}
}

func TestDetectProjectType_TypeScriptProject(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	// Create package.json file
	createTestFile(t, tmpDir, "package.json", `{"name": "test", "version": "1.0.0"}`)

	detector := NewProjectDetectorExecutor()
	info, err := detector.DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if info.Type != ProjectTypeTypeScript {
		t.Errorf("expected project type %q, got %q", ProjectTypeTypeScript, info.Type)
	}
	if info.BuildFile != "package.json" {
		t.Errorf("expected build file %q, got %q", "package.json", info.BuildFile)
	}
}

func TestDetectProjectType_PythonProject(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	// Create requirements.txt file
	createTestFile(t, tmpDir, "requirements.txt", "requests==2.31.0\n")

	detector := NewProjectDetectorExecutor()
	info, err := detector.DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if info.Type != ProjectTypePython {
		t.Errorf("expected project type %q, got %q", ProjectTypePython, info.Type)
	}
	if info.BuildFile != "requirements.txt" {
		t.Errorf("expected build file %q, got %q", "requirements.txt", info.BuildFile)
	}
}

func TestDetectProjectType_PyProjectToml(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	// Create pyproject.toml file
	createTestFile(t, tmpDir, "pyproject.toml", "[project]\nname = \"test\"\nversion = \"0.1.0\"\n")

	detector := NewProjectDetectorExecutor()
	info, err := detector.DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if info.Type != ProjectTypePython {
		t.Errorf("expected project type %q, got %q", ProjectTypePython, info.Type)
	}
}

func TestDetectProjectType_UnknownProject(t *testing.T) {
	tmpDir := createTempDir(t)
	defer cleanupTempDir(t, tmpDir)

	detector := NewProjectDetectorExecutor()
	_, err := detector.DetectProjectType(tmpDir)
	if err == nil {
		t.Fatal("expected error for unknown project type, got nil")
	}

	if !strings.Contains(err.Error(), "no known project type found") {
		t.Errorf("expected error message to contain 'no known project type found', got: %v", err)
	}
}

func TestDetectProjectType_NilDirectory(t *testing.T) {
	detector := NewProjectDetectorExecutor()
	_, err := detector.DetectProjectType("")
	if err == nil {
		t.Fatal("expected error for empty directory, got nil")
	}
}

// ============================================================================
// ProjectInfo Tests
// ============================================================================

func TestProjectInfo_String(t *testing.T) {
	info := &ProjectInfo{
		Type:      ProjectTypeGo,
		RootDir:   "/test/go",
		SrcDir:    ".",
		TestDir:   ".",
		BuildFile: "go.mod",
	}

	if info.Type != ProjectTypeGo {
		t.Errorf("expected type %q, got %q", ProjectTypeGo, info.Type)
	}
	if info.RootDir != "/test/go" {
		t.Errorf("expected root dir %q, got %q", "/test/go", info.RootDir)
	}
}

func TestProjectTypes_AllDefined(t *testing.T) {
	expectedTypes := []ProjectType{
		ProjectTypeGo,
		ProjectTypeKotlin,
		ProjectTypeJava,
		ProjectTypeRust,
		ProjectTypeTypeScript,
		ProjectTypePython,
		ProjectTypeUnknown,
	}

	for _, expectedType := range expectedTypes {
		if string(expectedType) == "" {
			t.Errorf("expected project type %q to have non-empty string value", expectedType)
		}
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
