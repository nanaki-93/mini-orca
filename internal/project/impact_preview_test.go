package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildImpactPreviewUsesOnlyIndexImports(t *testing.T) {
	preview := BuildImpactPreview(&ProjectIndex{Files: []IndexFile{{Path: "cmd/main.go", Imports: []string{"example/internal/project"}}, {Path: "internal/project/index.go"}}}, "internal/project/index.go", "BuildIndex")
	if len(preview.References) != 1 || preview.References[0].Path != "cmd/main.go" || preview.References[0].Confidence != "medium" {
		t.Fatalf("preview = %+v", preview)
	}
}

func TestReadGitStatusSupportsGitAndNonGitRoots(t *testing.T) {
	if status := ReadGitStatus(t.TempDir(), "main.go"); status.Available {
		t.Fatalf("non-git status = %+v", status)
	}
	root := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "fixture@example.test"}, {"config", "user.name", "Fixture"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if status := ReadGitStatus(root, "main.go"); !status.Available || status.Branch == "" {
		t.Fatalf("git status = %+v", status)
	}
}
