package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitWorkspaceListsOnlyCleanIncomingWorkAsMergeable(t *testing.T) {
	for _, state := range []string{"ready", "same head", "already merged", "dirty source", "dirty target", "locked", "detached", "unrelated"} {
		t.Run(state, func(t *testing.T) {
			service, root, request := gitWorkflowFixture(t)
			path := createWorkflowWorktree(t, service, request)
			if state != "same head" {
				testGit(t, path, "commit", "--allow-empty", "-m", "Incoming fix")
			}
			switch state {
			case "already merged":
				testGit(t, root, "merge", "--ff-only", "codex/fix")
				testGit(t, root, "commit", "--allow-empty", "-m", "Later current-branch work")
			case "dirty source", "dirty target":
				dirty := path
				if state == "dirty target" {
					dirty = root
				}
				if err := os.WriteFile(filepath.Join(dirty, "user.txt"), []byte("user work"), 0644); err != nil {
					t.Fatal(err)
				}
			case "locked":
				testGit(t, root, "worktree", "lock", path)
			case "detached":
				testGit(t, path, "switch", "--detach")
			case "unrelated":
				testGit(t, path, "switch", "--orphan", "unrelated")
				testGit(t, path, "commit", "--allow-empty", "-m", "Separate history")
			}
			workspace, err := service.GitWorkspace(context.Background(), request.ProjectID, request.ProjectRevision)
			if err != nil {
				t.Fatal(err)
			}
			for _, tree := range workspace.Worktrees {
				want := tree.Path == path && state == "ready"
				if tree.Mergeable != want {
					t.Fatalf("merge eligibility for %s: %+v", state, tree)
				}
			}
		})
	}
}

func TestDiscardWorktreeClosesImplementationWithoutMerging(t *testing.T) {
	service, root, request := gitWorkflowFixture(t)
	path := createWorkflowWorktree(t, service, request)
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("# Discarded task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testGit(t, path, "add", "README.md")
	testGit(t, path, "commit", "-m", "Task to discard")
	if _, _, err := service.WorktreeService(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	request.Path, request.SourceHead, request.SourceBranch = path, testGit(t, path, "rev-parse", "HEAD"), "codex/fix"
	result, err := service.DiscardGitWorktree(context.Background(), request)
	if err != nil || result.State != "discarded" || result.RemovedWorktree != path || result.Error != "" {
		t.Fatalf("discard: %+v, %v", result, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("discarded checkout still exists: %v", err)
	}
	if got := testGit(t, root, "rev-parse", "HEAD"); got != request.Head {
		t.Fatal("discard changed the current branch")
	}
	if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
		t.Fatal("discard applied the task to the current branch")
	}
	if got := testGit(t, root, "rev-parse", "refs/heads/codex/fix"); got != request.SourceHead {
		t.Fatal("discard deleted the retained branch")
	}
	if len(service.worktreeServices()) != 0 {
		t.Fatal("discard retained the implementation worker")
	}
}

func TestDiscardWorktreeRejectsChangedOrUnsafeSources(t *testing.T) {
	for _, state := range []string{"unconfirmed", "execution unconfirmed", "untrusted", "stale head", "stale branch", "dirty", "locked", "current", "nested"} {
		t.Run(state, func(t *testing.T) {
			service, root, request := gitWorkflowFixture(t)
			path := createWorkflowWorktree(t, service, request)
			request.Path, request.SourceHead, request.SourceBranch = path, request.Head, "codex/fix"
			switch state {
			case "unconfirmed":
				request.Confirm = false
			case "execution unconfirmed":
				request.ConfirmExecution = false
			case "untrusted":
				service.clearExecutionTrust()
			case "stale head":
				testGit(t, path, "commit", "--allow-empty", "-m", "Changed after confirmation")
			case "stale branch":
				testGit(t, path, "switch", "-c", "different")
			case "dirty":
				if err := os.WriteFile(filepath.Join(path, "user.txt"), []byte("keep me"), 0644); err != nil {
					t.Fatal(err)
				}
			case "locked":
				testGit(t, root, "worktree", "lock", path)
			case "current":
				request.Path, request.SourceBranch = root, request.Branch
			case "nested":
				child, _, err := service.WorktreeService(context.Background(), path)
				if err != nil {
					t.Fatal(err)
				}
				createWorkflowWorktreeWithBranch(t, child, "codex/nested", "nested")
			}
			result, err := service.DiscardGitWorktree(context.Background(), request)
			if state == "nested" {
				if err != nil || result.State != "failed" || result.RemovedWorktree != "" || !strings.Contains(result.Error, "another registered worktree") {
					t.Fatalf("nested discard: %+v, %v", result, err)
				}
			} else if err == nil {
				t.Fatalf("accepted %s discard: %+v", state, result)
			}
			for _, retained := range []string{root, path} {
				if _, err := os.Stat(retained); err != nil {
					t.Fatalf("removed protected checkout %s: %v", retained, err)
				}
			}
			if got := testGit(t, root, "rev-parse", "HEAD"); got != request.Head {
				t.Fatal("rejected discard changed the current branch")
			}
		})
	}
}
