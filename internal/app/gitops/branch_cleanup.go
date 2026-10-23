package gitops

import (
	"context"
	"fmt"
)

// DeleteMergedBranch removes only the captured ref, after its checkout has been
// removed. The compare-and-delete protects newer commits from concurrent Git work.
func DeleteMergedBranch(ctx context.Context, root string, source GitIdentity) error {
	if source.Branch == "" || !commitOIDPattern.MatchString(source.Head) {
		return fmt.Errorf("missing reviewed branch identity")
	}
	ref := "refs/heads/" + source.Branch
	if _, err := Read(ctx, root, "check-ref-format", ref); err != nil {
		return fmt.Errorf("validate reviewed branch: %w", err)
	}
	trees, err := ReadWorktrees(ctx, root)
	if err != nil {
		return fmt.Errorf("check branch checkouts: %w", err)
	}
	for _, tree := range trees {
		if tree.Branch == source.Branch {
			return fmt.Errorf("branch is still checked out at %s", tree.Path)
		}
	}
	if _, err := Read(ctx, root, "merge-base", "--is-ancestor", source.Head, "HEAD"); err != nil {
		return fmt.Errorf("verify branch commit is integrated: %w", err)
	}
	if _, err := Read(ctx, root, "update-ref", "--no-deref", "-d", ref, source.Head); err != nil {
		return fmt.Errorf("delete reviewed branch: %w", err)
	}
	return nil
}
