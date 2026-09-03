package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildFunctionWithManifestLimitsContextToSelectedDeclaration(t *testing.T) {
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
	file := index.Files[0]
	if file.Path != "helper.go" {
		file = index.Files[1]
	}
	context, manifest, err := NewContextBuilder().BuildFunctionWithManifest(root, FunctionContextOptions{
		TargetPath: "main.go", TargetSymbol: "Run", Mode: DeclarationEditReplaceSymbol, Index: index,
		TaskSpec:  &BugTaskSpec{SchemaVersion: BugTaskSpecSchemaVersion, TargetPath: "main.go", TargetSymbol: "Run", TargetSignature: file.Symbols[0].Signature, AcceptanceCriteria: []string{"Print the selected value."}, NonGoals: []string{"Do not edit helpers."}},
		MaxTokens: 400,
	})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.TokenLimit != 400 || len(manifest.Included) != 1 || manifest.Included[0].Path != "main.go" {
		t.Fatalf("manifest = %+v", manifest)
	}
	for _, unwanted := range []string{"helper-body", "other-body", "package sample\n\nimport"} {
		if strings.Contains(context, unwanted) {
			t.Fatalf("function context leaked unrelated source %q: %s", unwanted, context)
		}
	}
	if !strings.Contains(context, "func Run()") || !strings.Contains(context, "Print the selected value.") {
		t.Fatalf("function context omitted task boundary: %s", context)
	}
}

func TestBuildFunctionWithManifestSupportsCreateAndReportsTruncation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package sample\n\nfunc Run() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	index, err := BuildIndex(root, "project", "revision")
	if err != nil {
		t.Fatal(err)
	}
	created, manifest, err := NewContextBuilder().BuildFunctionWithManifest(root, FunctionContextOptions{TargetPath: "main.go", TargetSymbol: "NewRun", Mode: DeclarationEditCreateSymbol, Index: index, MaxTokens: 400})
	if err != nil || manifest.Truncated || !strings.Contains(created, "Create a new top-level declaration named NewRun.") {
		t.Fatalf("create context = %q, manifest = %+v, err = %v", created, manifest, err)
	}
	truncated, manifest, err := NewContextBuilder().BuildFunctionWithManifest(root, FunctionContextOptions{TargetPath: "main.go", TargetSymbol: "Run", Mode: DeclarationEditReplaceSymbol, Index: index, MaxTokens: 8})
	if err != nil || !manifest.Truncated || manifest.EstimatedTokens > manifest.TokenLimit || !strings.Contains(truncated, "[context truncated]") {
		t.Fatalf("truncated context = %q, manifest = %+v, err = %v", truncated, manifest, err)
	}
	_, manifest, err = NewContextBuilder().BuildFunctionWithManifest(root, FunctionContextOptions{TargetPath: "main.go", TargetSymbol: "Run", Mode: DeclarationEditReplaceSymbol, Index: index, MaxTokens: 1})
	if err != nil || !manifest.Truncated || manifest.EstimatedTokens > manifest.TokenLimit {
		t.Fatalf("tiny budget manifest = %+v, err = %v", manifest, err)
	}
}
