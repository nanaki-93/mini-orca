package app

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type AgentSkillSetting struct {
	Directory string `json:"directory"`
	Enabled   bool   `json:"enabled"`
}

type AgentSettings struct {
	ProjectID       string                  `json:"project_id"`
	ProjectRevision string                  `json:"project_revision"`
	Version         int64                   `json:"version"`
	Configuration   string                  `json:"configuration"`
	State           string                  `json:"state"`
	Roles           agentstate.RoleSettings `json:"roles"`
	Limits          config.AgentLimits      `json:"limits"`
	Ceilings        config.AgentLimits      `json:"ceilings"`
	Skills          []AgentSkillSetting     `json:"skills"`
	Models          []EffectiveModel        `json:"models"`
}

type AgentSettingsControl struct {
	ProjectID       string                       `json:"project_id"`
	ProjectRevision string                       `json:"project_revision"`
	Configuration   string                       `json:"configuration"`
	Version         int64                        `json:"version"`
	Action          string                       `json:"action"`
	Confirm         bool                         `json:"confirm"`
	Values          *agentstate.AgentPreferences `json:"values,omitempty"`
}

func runtimePreferences(runtime config.AgentRuntime) agentstate.AgentPreferences {
	return agentstate.AgentPreferences{Roles: agentstate.RoleSettings{Creation: string(runtime.Roles.Creation), Testing: string(runtime.Roles.Testing), Review: string(runtime.Roles.Review)}, Limits: runtime.Limits, DisabledSkills: []string{}}
}

func (s *Service) AgentSettings(ctx context.Context, query AgentRunQuery) (*AgentSettings, error) {
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	store, _, err := s.agentJournal(root)
	if err != nil {
		return nil, err
	}
	record, err := store.Settings(ctx)
	if err != nil {
		return nil, err
	}
	view, err := s.settingsView(query, record)
	if err == nil {
		err = s.integrationScope(ctx, query, root)
	}
	return view, err
}

func (s *Service) settingsView(query AgentRunQuery, record agentstate.SettingsRecord) (*AgentSettings, error) {
	configuration, err := s.agentConfiguration(s.agentRuntime)
	if err != nil {
		return nil, err
	}
	view := &AgentSettings{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Version: record.Version, Configuration: configuration, State: "configured", Ceilings: s.agentRuntime.Limits, Models: s.EffectiveModels(), Skills: []AgentSkillSetting{}}
	values := runtimePreferences(s.agentRuntime)
	if record.Values != nil {
		view.State, values = "overridden", *record.Values
		if record.Configuration != configuration || s.validateAgentPreferences(values) != nil {
			view.State = "stale"
		}
	}
	view.Roles, view.Limits = values.Roles, values.Limits
	for i := range view.Models {
		if value := view.Models[i].Temperature; value != nil {
			copy := *value
			view.Models[i].Temperature = &copy
		}
		if value := view.Models[i].MaxTokens; value != nil {
			copy := *value
			view.Models[i].MaxTokens = &copy
		}
	}
	for _, directory := range s.agentRuntime.Skills {
		view.Skills = append(view.Skills, AgentSkillSetting{Directory: directory, Enabled: !slices.Contains(values.DisabledSkills, directory)})
	}
	return view, nil
}

func (s *Service) validateAgentPreferences(values agentstate.AgentPreferences) error {
	if err := values.Validate(); err != nil {
		return err
	}
	limits, ceiling := values.Limits, s.agentRuntime.Limits
	for _, pair := range [][2]int64{
		{int64(limits.ModelCalls), int64(ceiling.ModelCalls)}, {int64(limits.ToolCalls), int64(ceiling.ToolCalls)},
		{limits.InputTokens, ceiling.InputTokens}, {limits.OutputTokens, ceiling.OutputTokens},
		{int64(limits.ElapsedSeconds), int64(ceiling.ElapsedSeconds)}, {int64(limits.ConcurrentCalls), int64(ceiling.ConcurrentCalls)},
		{int64(limits.Repairs), int64(ceiling.Repairs)},
	} {
		if pair[0] > pair[1] {
			return fmt.Errorf("project budgets cannot exceed configured ceilings")
		}
	}
	for _, directory := range values.DisabledSkills {
		if !slices.Contains(s.agentRuntime.Skills, directory) {
			return fmt.Errorf("skill controls must reference a configured directory")
		}
	}
	return nil
}

func (s *Service) validateSettingsControl(request AgentSettingsControl) error {
	if !request.Confirm || request.Action != "replace" && request.Action != "reset" {
		return fmt.Errorf("confirm replace or reset; settings affect future admissions")
	}
	if request.Action == "replace" {
		if request.Values == nil {
			return fmt.Errorf("replace requires complete roles, limits and disabled skills")
		}
		if err := s.validateAgentPreferences(*request.Values); err != nil {
			return err
		}
	} else if request.Values != nil {
		return fmt.Errorf("reset cannot include replacement values")
	}
	return nil
}

func (s *Service) ControlAgentSettings(ctx context.Context, request AgentSettingsControl) (*AgentSettings, error) {
	if err := s.validateSettingsControl(request); err != nil {
		return nil, err
	}
	// Reindex, project replacement and source Apply share this lifecycle lock.
	// The journal additionally serializes editors across service instances.
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	view, err := s.AgentSettings(ctx, query)
	if err != nil {
		return nil, err
	}
	if view.Version != request.Version || view.Configuration != request.Configuration || view.State == "stale" && request.Action != "reset" {
		return nil, fmt.Errorf("%w: reload settings; stale overrides require explicit reset", project.ErrRevisionConflict)
	}
	store, _, err := s.agentJournal(s.manager.Root())
	if err != nil {
		return nil, err
	}
	record, err := store.SaveSettings(ctx, request.Version, request.Configuration, request.Values)
	if errors.Is(err, agentstate.ErrSettingsConflict) {
		return nil, fmt.Errorf("%w: %v", project.ErrRevisionConflict, err)
	}
	if err != nil {
		return nil, err
	}
	s.cancelLiveSessions()
	s.notifyEvent("agent_settings")
	return s.settingsView(query, record)
}
