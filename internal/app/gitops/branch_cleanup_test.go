package gitops

import (
	"context"
	"strings"
	"testing"
)

func TestDeleteMergedBranchGuardsReviewedIdentityAndCheckouts(t *testing.T) {
	for _, scenario := range []string{"merged", "changed ref", "checked out", "current branch", "unmerged", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			runGitFixture(t, root, "init", "-b", "main")
			runGitFixture(t, root, "config", "user.email", "fixture@example.test")
			runGitFixture(t, root, "config", "user.name", "Fixture")
			runGitFixture(t, root, "commit", "--allow-empty", "-m", "Base")
			out, err := Read(context.Background(), root, "rev-parse", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			source := GitIdentity{Branch: "fix", Head: strings.TrimSpace(out)}
			runGitFixture(t, root, "branch", source.Branch)
			ctx := context.Background()
			switch scenario {
			case "changed ref", "unmerged":
				runGitFixture(t, root, "switch", "fix")
				runGitFixture(t, root, "commit", "--allow-empty", "-m", "New work")
				if scenario == "unmerged" {
					out, err := Read(ctx, root, "rev-parse", "HEAD")
					if err != nil {
						t.Fatal(err)
					}
					source.Head = strings.TrimSpace(out)
				}
				runGitFixture(t, root, "switch", "main")
			case "checked out":
				runGitFixture(t, root, "worktree", "add", t.TempDir(), "fix")
			case "current branch":
				source.Branch = "main"
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			err = DeleteMergedBranch(ctx, root, source)
			_, existsErr := Read(context.Background(), root, "show-ref", "--verify", "--quiet", "refs/heads/"+source.Branch)
			if scenario == "merged" {
				if err != nil || !ExitCode(existsErr, 1) {
					t.Fatalf("merged branch not removed: %v, ref: %v", err, existsErr)
				}
			} else if err == nil || existsErr != nil {
				t.Fatalf("unsafe branch removal: error=%v, ref=%v", err, existsErr)
			}
		})
	}
}
