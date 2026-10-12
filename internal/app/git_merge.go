package app

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type GitMergePreview struct {
	Target  GitIdentity `json:"target"`
	Source  GitIdentity `json:"source"`
	Path    string      `json:"path"`
	Diff    string      `json:"diff"`
	Commits string      `json:"commits"`
}

func mergeSource(ctx context.Context, repository *GitRepository, request GitMutationRequest) (*GitWorktree, error) {
	trees, err := readGitWorktrees(ctx, repository.Root)
	if err != nil {
		return nil, err
	}
	for _, tree := range trees {
		if tree.Path != request.Path {
			continue
		}
		if tree.Head != request.SourceHead {
			return nil, project.ErrRevisionConflict
		}
		source, err := readGitRepository(ctx, tree.Path)
		if err != nil {
			return nil, err
		}
		if err := requireGitMergeSource(&tree, source); err != nil {
			return nil, err
		}
		return &tree, nil
	}
	return nil, fmt.Errorf("choose an existing worktree from this repository")
}

func requireGitMergeSource(tree *GitWorktree, source *GitRepository) error {
	if tree.Current || tree.Locked || tree.Prunable || tree.Branch == "" {
		return project.ErrRevisionConflict
	}
	if err := requireGitIdentity(source, tree.GitIdentity); err != nil {
		return err
	}
	if source.Dirty {
		return fmt.Errorf("commit the worktree's remaining changes before merging or discarding it")
	}
	return nil
}

func requireGitMergeTarget(repository *GitRepository) error {
	if err := requireGitIdentity(repository, repository.GitIdentity); err != nil {
		return err
	}
	if repository.Dirty {
		return fmt.Errorf("commit or move local changes before merging")
	}
	return nil
}

// Merge eligibility is a passive guard, not a promise that Git will resolve conflicts.
func gitWorktreeMergeable(ctx context.Context, target *GitRepository, tree *GitWorktree, source *GitRepository) (bool, error) {
	if requireGitMergeTarget(target) != nil || requireGitMergeSource(tree, source) != nil {
		return false, nil
	}
	if _, err := gitRead(ctx, target.Root, "merge-base", target.Head, source.Head); err != nil {
		if gitExitCode(err, 1) {
			return false, nil
		}
		return false, err
	}
	_, err := gitRead(ctx, target.Root, "merge-base", "--is-ancestor", source.Head, target.Head)
	if gitExitCode(err, 1) {
		return true, nil
	}
	return false, err
}

func (s *Service) PreviewGitMerge(ctx context.Context, request GitMutationRequest) (*GitMergePreview, error) {
	repository, err := s.GitRepository(ctx, request.ProjectID, request.ProjectRevision)
	if err != nil {
		return nil, err
	}
	if err := requireGitIdentity(repository, request.GitIdentity); err != nil {
		return nil, err
	}
	source, err := mergeSource(ctx, repository, request)
	if err != nil {
		return nil, err
	}
	diff, err := gitRead(ctx, repository.Root, "diff", "--no-ext-diff", "--no-textconv", "--no-color", repository.Head+"..."+source.Head, "--")
	if err != nil {
		return nil, err
	}
	commits, err := gitRead(ctx, repository.Root, "log", "--format=%h %s", repository.Head+".."+source.Head, "--")
	if err != nil {
		return nil, err
	}
	return &GitMergePreview{Target: repository.GitIdentity, Source: source.GitIdentity, Path: source.Path, Diff: diff, Commits: commits}, nil
}

func (s *Service) MergeGitWorktree(ctx context.Context, request GitMutationRequest) (*GitMutationResult, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	repository, err := s.gitMutationPreflight(ctx, request, false)
	if err != nil {
		return nil, err
	}
	if err := requireGitMergeTarget(repository); err != nil {
		return nil, err
	}
	source, err := mergeSource(ctx, repository, request)
	if err != nil {
		return nil, err
	}
	s.invalidateJobsForProjectChange()
	_, err = gitRead(ctx, repository.Root, "merge", "--no-edit", "--no-stat", "--no-autostash", "--no-rerere-autoupdate", source.Head)
	result := s.completeGitMutation(repository.Root, "merged", err)
	if result.State == "merged" {
		if err := s.removeGitWorktree(ctx, repository.Root, source, worktreeMerged); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Merge completed, but worktree %s was not removed: %v. Resolve the issue and remove it with Git.", source.Path, err))
		} else {
			result.RemovedWorktree = source.Path
		}
	}
	return result, nil
}

func (s *Service) AbortGitMerge(ctx context.Context, request GitMutationRequest) (*GitMutationResult, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	repository, err := s.gitMutationPreflight(ctx, request, true)
	if err != nil {
		return nil, err
	}
	if repository.MergeHead == "" || repository.MergeHead != request.MergeHead {
		return nil, project.ErrRevisionConflict
	}
	s.invalidateJobsForProjectChange()
	_, err = gitRead(ctx, repository.Root, "merge", "--abort")
	return s.completeGitMutation(repository.Root, "aborted", err), nil
}
