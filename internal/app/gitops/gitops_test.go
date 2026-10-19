package gitops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStatusDirtyIgnoresPrivateMetadataAndWorktrees(t *testing.T) {
	if StatusDirty("?? .mini-orca/index.json\x00?? .worktrees/fix-1/\x00") {
		t.Fatal("private metadata or worktree paths marked repository dirty")
	}
	if !StatusDirty(" M main.go\x00") {
		t.Fatal("modified tracked file should mark repository dirty")
	}
}

func TestReadRepositoryOnNonGitDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	repo, err := ReadRepository(context.Background(), root)
	if err != nil {
		t.Fatalf("ReadRepository on non-git dir: %v", err)
	}
	if repo.Available {
		t.Fatalf("expected non-git directory to report Available=false, got %+v", repo)
	}
}
