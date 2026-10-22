package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/gitops"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// The caller owns jobLifecycleMu, so no new worktree request can be routed while
// its worker is stopped and its checkout is removed. Git retains the branch.
func (s *Service) removeGitWorktree(ctx context.Context, root string, source *GitWorktree, removal gitops.WorktreeRemoval) error {
	s.worktreesMu.Lock()
	child := s.worktrees[source.Path]
	s.worktreesMu.Unlock()
	if child != nil {
		child.jobLifecycleMu.Lock()
		defer child.jobLifecycleMu.Unlock()
		if err := gitops.ValidateWorktreeRemoval(ctx, root, source, removal); err != nil {
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
	if err := gitops.ValidateWorktreeRemoval(ctx, root, source, removal); err != nil {
		return err
	}
	// Private task history can be untracked even though the checkout is clean.
	// Ignore only that metadata so Git still refuses other uncommitted files.
	if err := gitops.AddLocalExclude(ctx, root, "/.mini-orca/"); err != nil {
		return err
	}
	if _, err := gitRead(ctx, root, "worktree", "remove", "--", source.Path); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(root, ".worktrees"))
	s.worktreesMu.Lock()
	delete(s.worktrees, source.Path)
	s.worktreesMu.Unlock()
	return nil
}

func (s *Service) stopWorktreeJobs(ctx context.Context) error {
	if err := s.ShutdownChangeWorkflows(ctx); err != nil {
		return err
	}
	s.goScan.mu.Lock()
	scanning := s.goScan.cancel != nil
	s.goScan.mu.Unlock()
	if scanning {
		return fmt.Errorf("wait for the project scan to finish stopping")
	}
	return nil
}

func (s *Service) DiscardGitWorktree(ctx context.Context, request GitMutationRequest) (*GitMutationResult, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	repository, err := s.gitMutationPreflight(ctx, request, false)
	if err != nil {
		return nil, err
	}
	source, err := gitops.MergeSource(ctx, repository, request)
	if err != nil {
		return nil, err
	}
	if request.SourceBranch == "" || source.Branch != request.SourceBranch {
		return nil, fmt.Errorf("worktree branch changed; refresh Git status before discarding: %w", project.ErrRevisionConflict)
	}
	targets := collectSessionFindingTargets(source.Path, false)
	result := &GitMutationResult{State: "discarded", Path: source.Path, Repository: repository, Warnings: []string{}}
	if err := s.removeGitWorktree(ctx, repository.Root, source, gitops.WorktreeDiscarded); err != nil {
		result.State, result.Error = "failed", err.Error()
	} else {
		result.RemovedWorktree = source.Path
		s.reopenDiscardedWorktreeFindings(ctx, repository.Root, repository.ProjectID, targets)
	}
	return result, nil
}

func (s *Service) reopenDiscardedWorktreeFindings(ctx context.Context, root, projectID string, targets []project.FindingTarget) {
	if len(targets) == 0 {
		return
	}
	active := s.inProgressFindingTargets(ctx, root, projectID)
	stillActive := make(map[string]bool, len(active))
	for _, t := range active {
		stillActive[t.ID] = true
	}
	discarded := make(map[string]bool, len(targets))
	for _, t := range targets {
		if !stillActive[t.ID] {
			discarded[t.ID] = true
		}
	}
	if len(discarded) == 0 {
		return
	}
	store, err := project.NewFindingStore(root)
	if err != nil {
		return
	}
	stored, err := store.Load(project.FindingInput{ProjectID: projectID})
	if err != nil {
		return
	}
	for _, f := range stored {
		if f.Status == project.FindingStatusInProgress && discarded[f.ID] {
			_ = store.SetStatus(f.ID, project.FindingStatusOpen)
		}
	}
}
