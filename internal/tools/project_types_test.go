package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProjectType_Go(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	pt, err := DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pt != ProjectTypeGo {
		t.Errorf("expected project type '%s', got '%s'", ProjectTypeGo, pt)
	}
}

func TestDetectProjectType_Kotlin(t *testing.T) {
	tmpDir := t.TempDir()

	// Create build.gradle.kts
	err := os.WriteFile(filepath.Join(tmpDir, "build.gradle.kts"), []byte("plugins { id('org.jetbrains.kotlin.jvm') }"), 0644)
	if err != nil {
		t.Fatalf("failed to create build.gradle.kts: %v", err)
	}

	pt, err := DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pt != ProjectTypeKotlin {
		t.Errorf("expected project type '%s', got '%s'", ProjectTypeKotlin, pt)
	}
}

func TestDetectProjectType_Rust(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Cargo.toml
	err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]\nname = \"test\"\nversion = \"0.1.0\"\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create Cargo.toml: %v", err)
	}

	pt, err := DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pt != ProjectTypeRust {
		t.Errorf("expected project type '%s', got '%s'", ProjectTypeRust, pt)
	}
}

func TestDetectProjectType_TypeScript(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.json
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"name": "test", "dependencies": {"typescript": "^5.0.0"}}`), 0644)
	if err != nil {
		t.Fatalf("failed to create package.json: %v", err)
	}

	pt, err := DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pt != ProjectTypeTypeScript {
		t.Errorf("expected project type '%s', got '%s'", ProjectTypeTypeScript, pt)
	}
}

func TestDetectProjectType_Python(t *testing.T) {
	tmpDir := t.TempDir()

	// Create requirements.txt
	err := os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte("pytest\nrequests\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create requirements.txt: %v", err)
	}

	pt, err := DetectProjectType(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pt != ProjectTypePython {
		t.Errorf("expected project type '%s', got '%s'", ProjectTypePython, pt)
	}
}

func TestDetectProjectType_Unknown(t *testing.T) {
	tmpDir := t.TempDir()

	pt, err := DetectProjectType(tmpDir)
	if err == nil {
		t.Error("expected error for unknown project type")
	}
	if pt != ProjectTypeUnknown {
		t.Errorf("expected project type '%s', got '%s'", ProjectTypeUnknown, pt)
	}
}

func TestProjectTypeIsSupported(t *testing.T) {
	tests := []struct {
		pt        ProjectType
		supported bool
	}{
		{ProjectTypeGo, true},
		{ProjectTypeKotlin, true},
		{ProjectTypeJava, true},
		{ProjectTypeRust, true},
		{ProjectTypeTypeScript, true},
		{ProjectTypePython, true},
		{ProjectTypeUnknown, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.pt), func(t *testing.T) {
			if got := tt.pt.IsSupported(); got != tt.supported {
				t.Errorf("IsSupported() = %v, want %v", got, tt.supported)
			}
		})
	}
}

func TestProjectTypeString(t *testing.T) {
	tests := []struct {
		pt   ProjectType
		want string
	}{
		{ProjectTypeGo, "go"},
		{ProjectTypeKotlin, "kotlin"},
		{ProjectTypeJava, "java"},
		{ProjectTypeRust, "rust"},
		{ProjectTypeTypeScript, "typescript"},
		{ProjectTypePython, "python"},
	}

	for _, tt := range tests {
		t.Run(string(tt.pt), func(t *testing.T) {
			if got := tt.pt.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewExecutorWithProjectType(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewExecutorWithProjectType(tmpDir, ProjectTypeGo)

	if executor == nil {
		t.Fatal("expected non-nil executor")
	}
	if executor.ProjectType() != ProjectTypeGo {
		t.Errorf("expected project type '%s', got '%s'", ProjectTypeGo, executor.ProjectType())
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if !fileExists(testFile) {
		t.Error("expected file to exist")
	}

	if fileExists(filepath.Join(tmpDir, "nonexistent.txt")) {
		t.Error("expected file to not exist")
	}
}

func TestDirExists(t *testing.T) {
	tmpDir := t.TempDir()

	if !dirExists(tmpDir) {
		t.Error("expected directory to exist")
	}

	if dirExists(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("expected directory to not exist")
	}
}

func TestAnalyzeProject_Go(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	info, err := AnalyzeProject(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if info.Type != ProjectTypeGo {
		t.Errorf("expected type '%s', got '%s'", ProjectTypeGo, info.Type)
	}
	if info.RootDir != tmpDir {
		t.Errorf("expected root dir '%s', got '%s'", tmpDir, info.RootDir)
	}
	if info.TestPattern != "./..." {
		t.Errorf("expected test pattern './...', got '%s'", info.TestPattern)
	}
}

func TestAnalyzeProject_Rust(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Cargo.toml
	err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]\nname = \"test\"\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create Cargo.toml: %v", err)
	}

	info, err := AnalyzeProject(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if info.Type != ProjectTypeRust {
		t.Errorf("expected type '%s', got '%s'", ProjectTypeRust, info.Type)
	}
	if info.SourceDir != filepath.Join(tmpDir, "src") {
		t.Errorf("expected source dir '%s', got '%s'", filepath.Join(tmpDir, "src"), info.SourceDir)
	}
}

func TestAnalyzeProject_KotlinWithGradle(t *testing.T) {
	tmpDir := t.TempDir()

	// Create build.gradle.kts
	err := os.WriteFile(filepath.Join(tmpDir, "build.gradle.kts"), []byte("plugins { kotlin('jvm') }"), 0644)
	if err != nil {
		t.Fatalf("failed to create build.gradle.kts: %v", err)
	}

	info, err := AnalyzeProject(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if info.Type != ProjectTypeKotlin {
		t.Errorf("expected type '%s', got '%s'", ProjectTypeKotlin, info.Type)
	}
}

func TestAnalyzeProject_Unknown(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := AnalyzeProject(tmpDir)
	if err == nil {
		t.Error("expected error for unknown project type")
	}
}

func TestExecutor_ProjectType(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewExecutorWithProjectType(tmpDir, ProjectTypePython)

	if executor.ProjectType() != ProjectTypePython {
		t.Errorf("expected project type '%s', got '%s'", ProjectTypePython, executor.ProjectType())
	}
}

// BenchmarkDetectProjectType benchmarks project type detection
func BenchmarkDetectProjectType_Go(b *testing.B) {
	tmpDir := b.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DetectProjectType(tmpDir)
	}
}

// BenchmarkAnalyzeProject benchmarks project analysis
func BenchmarkAnalyzeProject_Go(b *testing.B) {
	tmpDir := b.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = AnalyzeProject(tmpDir)
	}
}

// BenchmarkProjectTypeIsSupported benchmarks IsSupported check
func BenchmarkProjectTypeIsSupported(b *testing.B) {
	pt := ProjectTypeGo
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pt.IsSupported()
	}
}
