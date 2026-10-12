package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// The caller owns jobLifecycleMu, so no new worktree request can be routed while
// its worker is stopped and its checkout is removed. Git retains the branch.
func (s *Service) removeMergedWorktree(ctx context.Context, root string, source *GitWorktree) error {
	s.worktreesMu.Lock()
	child := s.worktrees[source.Path]
	s.worktreesMu.Unlock()
	if child != nil {
		child.jobLifecycleMu.Lock()
		defer child.jobLifecycleMu.Unlock()
		if err := validateMergedWorktree(ctx, root, source); err != nil {
			return err
		}
		cleanup, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := child.stopWorktreeJobs(cleanup); err != nil {
			return fmt.Errorf("wait for implementation cleanup: %w", err)
		}
		child.changesMu.Lock()
		defer child.changesMu.Unlock()
	}
	if err := validateMergedWorktree(ctx, root, source); err != nil {
		return err
	}
	// Private task history can be untracked even though the checkout is clean.
	// Ignore only that metadata so Git still refuses other uncommitted files.
	if err := addGitLocalExclude(ctx, root, "/.mini-orca/"); err != nil {
		return err
	}
	if _, err := gitRead(ctx, root, "worktree", "remove", "--", source.Path); err != nil {
		return err
	}
	s.worktreesMu.Lock()
	delete(s.worktrees, source.Path)
	s.worktreesMu.Unlock()
	return nil
}

func (s *Service) stopWorktreeJobs(ctx context.Context) error {
	s.invalidateJobsForProjectChange()
	if err := s.ShutdownChangeWorkflows(ctx); err != nil {
		return err
	}
	s.analysisRun.mu.Lock()
	done := s.analysisRun.done
	s.analysisRun.mu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.goScan.mu.Lock()
	scanning := s.goScan.cancel != nil
	s.goScan.mu.Unlock()
	if scanning {
		return fmt.Errorf("wait for the project scan to finish stopping")
	}
	return nil
}

func validateMergedWorktree(ctx context.Context, root string, source *GitWorktree) error {
	repository, err := readGitRepository(ctx, root)
	if err != nil {
		return err
	}
	if err := requireGitIdentity(repository, repository.GitIdentity); err != nil {
		return err
	}
	if _, err := gitRead(ctx, root, "merge-base", "--is-ancestor", source.Head, repository.Head); err != nil {
		return fmt.Errorf("verify the reviewed commit is merged: %w", err)
	}
	current, err := mergeSource(ctx, repository, GitMutationRequest{Path: source.Path, SourceHead: source.Head})
	if err != nil {
		return err
	}
	if current.GitIdentity != source.GitIdentity {
		return fmt.Errorf("the worktree branch changed after review")
	}
	trees, err := readGitWorktrees(ctx, root)
	if err != nil {
		return err
	}
	for _, tree := range trees {
		if tree.Path == source.Path {
			continue
		}
		relative, err := filepath.Rel(source.Path, tree.Path)
		if err != nil {
			return err
		}
		if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("the checkout contains another registered worktree at %s", tree.Path)
		}
	}
	return nil
}
