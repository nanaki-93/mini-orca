package app

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/app/gitops"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type GitMergePreview = gitops.GitMergePreview

func (s *Service) PreviewGitMerge(ctx context.Context, request GitMutationRequest) (*GitMergePreview, error) {
	repository, err := s.GitRepository(ctx, request.ProjectID, request.ProjectRevision)
	if err != nil {
		return nil, err
	}
	return gitops.PreviewMerge(ctx, repository, request)
}

func (s *Service) MergeGitWorktree(ctx context.Context, request GitMutationRequest) (*GitMutationResult, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	repository, err := s.gitMutationPreflight(ctx, request, false)
	if err != nil {
		return nil, err
	}
	if err := gitops.RequireMergeTarget(repository); err != nil {
		return nil, err
	}
	source, err := gitops.MergeSource(ctx, repository, request)
	if err != nil {
		return nil, err
	}
	completed, err := gitops.MergedFindingTargets(ctx, repository, source, listChangeSessions)
	if err != nil {
		return nil, err
	}
	s.invalidateJobsForProjectChange()
	_, err = gitRead(ctx, repository.Root, "merge", "--no-edit", "--no-stat", "--no-autostash", "--no-rerere-autoupdate", source.Head)
	result := s.completeGitMutation(repository.Root, "merged", err)
	if result.State == "merged" {
		s.completeMergedWorktree(ctx, repository.Root, source, completed, result)
	}
	return result, nil
}

// CleanupMergedWorktree reconciles a merge completed outside Mini-Orca. It never
// attempts a merge or deletes a branch; ancestry must prove integration first.
func (s *Service) CleanupMergedWorktree(ctx context.Context, request GitMutationRequest) (*GitMutationResult, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	repository, err := s.gitMutationPreflight(ctx, request, false)
	if err != nil {
		return nil, err
	}
	if err := gitops.RequireMergeTarget(repository); err != nil {
		return nil, err
	}
	source, err := gitops.MergeSource(ctx, repository, request)
	if err != nil {
		return nil, err
	}
	if request.SourceBranch == "" || source.Branch != request.SourceBranch {
		return nil, project.ErrRevisionConflict
	}
	if err := requireFinishedWorktreeTasks(source.Path); err != nil {
		return nil, err
	}
	if err := gitops.ValidateWorktreeRemoval(ctx, repository.Root, source, gitops.WorktreeMerged); err != nil {
		return nil, err
	}
	completed, err := gitops.MergedFindingTargets(ctx, repository, source, listChangeSessions)
	if err != nil {
		return nil, err
	}
	s.invalidateJobsForProjectChange()
	result := s.completeGitMutation(repository.Root, "merged", nil)
	s.completeMergedWorktree(ctx, repository.Root, source, completed, result)
	return result, nil
}

func requireFinishedWorktreeTasks(path string) error {
	history, err := listChangeSessions(path)
	if err != nil {
		return err
	}
	for _, entry := range history {
		if entry.State != "applied" || entry.Commit == nil || entry.Commit.State != "committed" {
			return fmt.Errorf("worktree contains unfinished task %q; finish it or explicitly discard the worktree", entry.Title)
		}
	}
	return nil
}

func (s *Service) completeMergedWorktree(ctx context.Context, root string, source *GitWorktree, completed []project.FindingCompletion, result *GitMutationResult) {
	if err := gitops.CompleteMergedFindings(ctx, root, source.Head, completed); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Merge completed, but findings could not be marked Done: %v. Worktree %s and its task history were retained.", err, source.Path))
		return
	}
	if err := s.removeGitWorktree(ctx, root, source, gitops.WorktreeMerged); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Merge completed, but worktree %s was not removed: %v. Refresh Worktrees and retry cleanup after resolving the issue.", source.Path, err))
	} else {
		result.RemovedWorktree = source.Path
	}
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
