package gitops

import (
	"context"
	"strings"
)

// InspectIntegration compares committed paths without running merge drivers or
// writing a merge result. Overlap indicates review risk, not a proven conflict.
func InspectIntegration(ctx context.Context, target *GitRepository, tree *GitWorktree) error {
	if tree.Current {
		tree.Integration = "current"
		return nil
	}
	base, err := Read(ctx, target.Root, "merge-base", target.Head, tree.Head)
	if ExitCode(err, 1) {
		tree.Integration = "unrelated"
		return nil
	}
	if err != nil {
		return err
	}
	base = strings.TrimSpace(base)
	switch base {
	case tree.Head:
		tree.Integration = "merged"
	case target.Head:
		tree.Integration = "up_to_date"
	default:
		tree.Integration = "behind"
		targetPaths, err := changedPaths(ctx, target.Root, base, target.Head)
		if err != nil {
			return err
		}
		sourcePaths, err := changedPaths(ctx, target.Root, base, tree.Head)
		if err != nil {
			return err
		}
		changed := make(map[string]bool, len(targetPaths))
		for _, path := range targetPaths {
			changed[path] = true
		}
		for _, path := range sourcePaths {
			if changed[path] {
				tree.OverlapPaths = append(tree.OverlapPaths, path)
			}
		}
	}
	return nil
}

func changedPaths(ctx context.Context, root, base, head string) ([]string, error) {
	out, err := Read(ctx, root, "diff", "--name-only", "--no-renames", "--no-ext-diff", "--no-textconv", "-z", base, head, "--")
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(out, "\x00"), "\x00"), nil
}
