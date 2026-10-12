package app

import (
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func (s *Service) analysisInputs() (*project.Analysis, *project.ProjectIndex, *project.ContextPolicy, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, nil, nil, err
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, nil, nil, err
	}
	if index.ProjectID != analysis.ProjectID || index.ProjectRevision != analysis.ProjectRevision {
		return nil, nil, nil, project.ErrRevisionConflict
	}
	policy, err := project.NewContextPolicy(s.manager.Root())
	if err != nil {
		return nil, nil, nil, err
	}
	return analysis, index, policy, nil
}
