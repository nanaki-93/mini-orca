package app

import (
	"context"

	"github.com/nanaki-93/mini-orca/v2/internal/app/gitops"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type GitIdentity = gitops.GitIdentity
type GitRepository = gitops.GitRepository

func (s *Service) GitRepository(ctx context.Context, id, revision string) (*GitRepository, error) {
	root := s.manager.Root()
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if index.ProjectID != id || index.ProjectRevision != revision {
		return nil, project.ErrRevisionConflict
	}
	repository, err := readGitRepository(ctx, root)
	if err != nil {
		return nil, err
	}
	current, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if s.manager.Root() != root || current.ProjectID != id || current.ProjectRevision != revision {
		return nil, project.ErrRevisionConflict
	}
	repository.ProjectID, repository.ProjectRevision = id, revision
	return repository, nil
}

func readGitRepository(ctx context.Context, root string) (*GitRepository, error) {
	return gitops.ReadRepository(ctx, root)
}

func requireGitIdentity(repository *GitRepository, expected GitIdentity) error {
	return gitops.RequireIdentity(repository, expected)
}

func captureChangeGit(ctx context.Context, root string) (*GitIdentity, error) {
	return gitops.CaptureChangeGit(ctx, root)
}

func verifyChangeGit(ctx context.Context, root string, expected *GitIdentity) error {
	return gitops.VerifyChangeGit(ctx, root, expected)
}
