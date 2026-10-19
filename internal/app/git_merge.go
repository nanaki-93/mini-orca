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
		if err := gitops.CompleteMergedFindings(ctx, repository.Root, source.Head, completed); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Merge completed, but findings could not be marked Done: %v. Worktree %s and its task history were retained.", err, source.Path))
			return result, nil
		}
		if err := s.removeGitWorktree(ctx, repository.Root, source, gitops.WorktreeMerged); err != nil {
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
