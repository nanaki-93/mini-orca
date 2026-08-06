package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Project Detection Tests
// ============================================================================

func TestDetectProjectType_GoProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod file
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module example.com/test\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

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
	tmpDir := t.TempDir()

	// Create build.gradle file
	gradlePath := filepath.Join(tmpDir, "build.gradle")
	if err := os.WriteFile(gradlePath, []byte("plugins { id 'java' }\n"), 0644); err != nil {
		t.Fatalf("failed to create build.gradle: %v", err)
	}

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
	tmpDir := t.TempDir()

	// Create build.gradle.kts file
	gradleKtsPath := filepath.Join(tmpDir, "build.gradle.kts")
	if err := os.WriteFile(gradleKtsPath, []byte("plugins { id(\"org.jetbrains.kotlin.jvm\") version \"1.9.0\" }\n"), 0644); err != nil {
		t.Fatalf("failed to create build.gradle.kts: %v", err)
	}

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
	tmpDir := t.TempDir()

	// Create Cargo.toml file
	cargoPath := filepath.Join(tmpDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte("[package]\nname = \"test\"\nversion = \"0.1.0\"\n"), 0644); err != nil {
		t.Fatalf("failed to create Cargo.toml: %v", err)
	}

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
	tmpDir := t.TempDir()

	// Create package.json file
	packagePath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(packagePath, []byte(`{"name": "test", "version": "1.0.0"}`), 0644); err != nil {
		t.Fatalf("failed to create package.json: %v", err)
	}

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
	tmpDir := t.TempDir()

	// Create requirements.txt file
	reqPath := filepath.Join(tmpDir, "requirements.txt")
	if err := os.WriteFile(reqPath, []byte("requests==2.31.0\n"), 0644); err != nil {
		t.Fatalf("failed to create requirements.txt: %v", err)
	}

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
	tmpDir := t.TempDir()

	// Create pyproject.toml file
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte("[project]\nname = \"test\"\nversion = \"0.1.0\"\n"), 0644); err != nil {
		t.Fatalf("failed to create pyproject.toml: %v", err)
	}

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
	tmpDir := t.TempDir()

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
