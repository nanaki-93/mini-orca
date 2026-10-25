package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildFunctionWithManifestLimitsContextToSelectedFile(t *testing.T) {
	root := t.TempDir()
	target := "package sample\n\nimport \"fmt\"\n\nfunc Run() { fmt.Println(\"selected\") }\n\nfunc Other() { println(\"other-body\") }\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(target), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "helper.go"), []byte("package sample\n\nfunc Helper() { println(\"helper-body\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	index, err := BuildIndex(root, "project", "revision")
	if err != nil {
		t.Fatal(err)
	}
	context, manifest, err := NewContextBuilder().BuildFunctionWithManifest(root, FunctionContextOptions{
		TargetPath: "main.go", Index: index,
		MaxTokens: 400,
	})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.TokenLimit != 400 || len(manifest.Included) != 1 || manifest.Included[0].Path != "main.go" {
		t.Fatalf("manifest = %+v", manifest)
	}
	if strings.Contains(context, "helper-body") {
		t.Fatalf("file context leaked unrelated file source: %s", context)
	}
	if !strings.Contains(context, "func Run()") {
		t.Fatalf("file context omitted target source: %s", context)
	}
}

func TestBuildFunctionWithManifestRejectsUnindexedFilesAndReportsTruncation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package sample\n\nfunc Run() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	index, err := BuildIndex(root, "project", "revision")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := NewContextBuilder().BuildFunctionWithManifest(root, FunctionContextOptions{TargetPath: "missing.go", Index: index}); err == nil {
		t.Fatal("unindexed file was accepted")
	}
	truncated, manifest, err := NewContextBuilder().BuildFunctionWithManifest(root, FunctionContextOptions{TargetPath: "main.go", Index: index, MaxTokens: 8})
	if err != nil || !manifest.Truncated || manifest.EstimatedTokens > manifest.TokenLimit || !strings.Contains(truncated, "[context truncated]") {
		t.Fatalf("truncated context = %q, manifest = %+v, err = %v", truncated, manifest, err)
	}
	_, manifest, err = NewContextBuilder().BuildFunctionWithManifest(root, FunctionContextOptions{TargetPath: "main.go", Index: index, MaxTokens: 1})
	if err != nil || !manifest.Truncated || manifest.EstimatedTokens > manifest.TokenLimit {
		t.Fatalf("tiny budget manifest = %+v, err = %v", manifest, err)
	}
}

func TestBuildFunctionWithManifestIncludesScopedInstructions(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package sample\n\nfunc Run() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Preserve cancellation and test error paths."), 0600); err != nil {
		t.Fatal(err)
	}
	index, err := BuildIndex(root, "project", "revision")
	if err != nil {
		t.Fatal(err)
	}
	text, manifest, err := NewContextBuilder().BuildFunctionWithManifest(root, FunctionContextOptions{TargetPath: "main.go", Index: index})
	if err != nil || !strings.Contains(text, "Preserve cancellation") || len(manifest.Included) != 2 || manifest.Included[0].Path != "AGENTS.md" {
		t.Fatalf("instructions absent from context: %q, %+v, %v", text, manifest, err)
	}
}
