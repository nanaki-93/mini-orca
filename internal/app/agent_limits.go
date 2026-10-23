package app

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
)

// Admission selects destinations before workers start. Per-attempt source and
// consent guards still run in their workflow owner immediately before dispatch.
func (s *Service) agentRunContext(ctx context.Context, runtimes ...modelRuntime) (context.Context, context.CancelFunc, error) {
	clients := make([]*agentruntime.ProviderClient, 0, len(runtimes))
	for _, runtime := range runtimes {
		clients = append(clients, runtime.client)
	}
	admitted, err := agentruntime.WithPolicy(ctx, clients...)
	if err != nil {
		return nil, nil, fmt.Errorf("admit agent destinations: %w", err)
	}
	return agentruntime.WithBudget(admitted, s.agentRuntime.Limits)
}

func (s *Service) workflowAgentContext(ctx context.Context, models ChangeWorkflowModels) (context.Context, context.CancelFunc, error) {
	runtimes := make([]modelRuntime, 0, 3)
	for stage, profile := range models.selections() {
		runtimes = append(runtimes, s.workflowStageRuntime(stage, profile))
	}
	return s.agentRunContext(ctx, runtimes...)
}

func (s *Service) analysisAgentContext(ctx context.Context, preview *AnalysisRunPreview) (context.Context, context.CancelFunc, error) {
	runtimes := make([]modelRuntime, 0, 4)
	for _, provider := range preview.Providers {
		for _, stage := range provider.Stages {
			runtime := s.analysisModelRuntime(stage, preview.Models)
			// Missing models remain unavailable in the stage/recovery owner.
			// They receive no grant and cannot silently acquire a fallback.
			if runtime.client != nil {
				runtimes = append(runtimes, runtime)
			}
		}
	}
	return s.agentRunContext(ctx, runtimes...)
}

func (s *Service) resolveWorkflowModels(models ChangeWorkflowModels) ChangeWorkflowModels {
	roles := s.agentRuntime.Roles
	if models.Create == "" {
		models.Create = string(roles.Creation)
	}
	if models.Test == "" {
		models.Test = string(roles.Testing)
	}
	if models.Review == "" {
		models.Review = string(roles.Review)
	}
	return models
}
