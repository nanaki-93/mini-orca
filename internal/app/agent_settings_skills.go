package app

import (
	"context"
	"fmt"
	"slices"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type AgentSkillValidationRequest struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	Configuration   string `json:"configuration"`
	Version         int64  `json:"version"`
	Directory       string `json:"directory"`
}

type AgentSkillValidation struct {
	AgentSkillValidationRequest
	State      string                        `json:"state"`
	Inspection *agentruntime.SkillInspection `json:"inspection"`
}

func (s *Service) ValidateAgentSkill(ctx context.Context, request AgentSkillValidationRequest) (*AgentSkillValidation, error) {
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	view, err := s.AgentSettings(ctx, query)
	if err != nil {
		return nil, err
	}
	if view.Version != request.Version || view.Configuration != request.Configuration || view.State == "stale" {
		return nil, project.ErrRevisionConflict
	}
	if !slices.Contains(s.conf().agentRuntime.Skills, request.Directory) {
		return nil, fmt.Errorf("validation requires a configured skill directory")
	}
	root := s.manager.Root()
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	input, err := captureSkillPackage(index, request.Directory, policy)
	if err != nil {
		return nil, err
	}
	inspection, err := agentruntime.InspectSkill(ctx, input, s.skillInspectionReader(root, query, policy.Version()))
	if err != nil {
		return nil, err
	}
	current, err := s.AgentSettings(ctx, query)
	if err != nil {
		return nil, err
	}
	if current.Version != view.Version || current.Configuration != view.Configuration || s.manager.Root() != root {
		return nil, project.ErrRevisionConflict
	}
	return &AgentSkillValidation{AgentSkillValidationRequest: request, State: "valid", Inspection: inspection}, nil
}

func (s *Service) skillInspectionReader(root string, query AgentRunQuery, policy string) func(context.Context, string, string) ([]byte, error) {
	return func(ctx context.Context, path, hash string) ([]byte, error) {
		if err := s.agentScope(ctx, query, root); err != nil {
			return nil, err
		}
		current, err := project.NewContextPolicy(root)
		if err != nil {
			return nil, err
		}
		if current.Version() != policy {
			return nil, project.ErrRevisionConflict
		}
		return current.ReadSource(ctx, path, hash)
	}
}
