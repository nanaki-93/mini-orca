package app

import (
	"context"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// validateSourceFileSnapshot rechecks the identity and policy facts captured
// before source-only or provider-backed file work is published.
func (s *Service) validateSourceFileSnapshot(ctx context.Context, root string, analysis project.Analysis, file project.IndexFile, policyVersion string, requireText bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.manager.Root() != root {
		return project.ErrRevisionConflict
	}
	currentAnalysis, err := s.manager.Analysis()
	if err != nil {
		return err
	}
	if currentAnalysis.ProjectID != analysis.ProjectID || currentAnalysis.ProjectRevision != analysis.ProjectRevision {
		return project.ErrRevisionConflict
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return err
	}
	if policy.Version() != policyVersion || !policy.Decide(file.Path).Include {
		return project.ErrRevisionConflict
	}
	info, err := project.GetFileInfo(root, file.Path)
	if err != nil {
		return err
	}
	if info.ContentHash != file.ContentHash || requireText && info.Binary {
		return project.ErrRevisionConflict
	}
	return nil
}
