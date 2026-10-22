package gitops

import (
	"context"
	"errors"
	"os"
	"os/exec"
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

func TestReadFileStatusSupportsGitAndNonGitRoots(t *testing.T) {
	if status, err := ReadFileStatus(context.Background(), t.TempDir(), "main.go"); err != nil || status.Available {
		t.Fatalf("non-git status = %+v, err = %v", status, err)
	}
	root := t.TempDir()
	runGitFixture(t, root, "init")
	runGitFixture(t, root, "config", "user.email", "fixture@example.test")
	runGitFixture(t, root, "config", "user.name", "Fixture")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	status, err := ReadFileStatus(context.Background(), root, "main.go")
	if err != nil || !status.Available || status.Branch == "" {
		t.Fatalf("git status = %+v, err = %v", status, err)
	}
}

func TestReadFileStatusNeutralizesFsmonitorFiltersAndHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	runGitFixture(t, root, "init")
	runGitFixture(t, root, "config", "user.email", "fixture@example.test")
	runGitFixture(t, root, "config", "user.name", "Fixture")
	mainPath := filepath.Join(root, "main.go")
	if err := os.WriteFile(mainPath, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.go filter=evil diff=evil\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, root, "add", "main.go", ".gitattributes")
	runGitFixture(t, root, "commit", "-m", "initial")
	if err := os.WriteFile(mainPath, []byte("package main\nfunc Changed() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	hookDir := t.TempDir()
	marker := filepath.Join(hookDir, "executed")
	hookScript := filepath.Join(hookDir, "hook.sh")
	if err := os.WriteFile(hookScript, []byte("#!/bin/sh\necho ran > "+marker+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, root, "config", "core.fsmonitor", hookScript)
	runGitFixture(t, root, "config", "filter.evil.clean", hookScript)
	runGitFixture(t, root, "config", "diff.evil.command", hookScript)

	status, err := ReadFileStatus(context.Background(), root, "main.go")
	if err != nil {
		t.Fatalf("ReadFileStatus error = %v", err)
	}
	if !status.Available || status.DiffState != "modified" {
		t.Fatalf("unexpected status = %+v", status)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("passive file Git status executed repository hook/filter: %v", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadFileStatus(canceled, root, "main.go"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func runGitFixture(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
