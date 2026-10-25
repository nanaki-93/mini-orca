package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

var ErrMissionPlanningIncomplete = agentruntime.ErrMissionPlanningIncomplete

type MissionPlanningRequest struct {
	ProjectID             string                     `json:"project_id"`
	ProjectRevision       string                     `json:"project_revision"`
	CatalogRevision       int64                      `json:"catalog_revision"`
	Goal                  string                     `json:"goal"`
	Sources               []agentstate.MissionSource `json:"sources"`
	Limits                agentstate.MissionLimits   `json:"limits"`
	PlanningLimits        agentstate.MissionLimits   `json:"planning_limits"`
	ConfirmRemoteProvider bool                       `json:"confirm_remote_provider"`
}

func (request *MissionPlanningRequest) UnmarshalJSON(data []byte) error {
	type plain MissionPlanningRequest
	var value plain
	if err := decodeMissionJSON(data, &value); err != nil {
		return err
	}
	*request = MissionPlanningRequest(value)
	return nil
}

type MissionPlanningResult struct {
	AgentMission
	InputHash       string                   `json:"input_hash"`
	PromptVersion   string                   `json:"prompt_version"`
	PlanningLimits  agentstate.MissionLimits `json:"planning_limits"`
	Usage           agentstate.BudgetUsage   `json:"usage"`
	ContextManifest project.ContextManifest  `json:"context_manifest"`
}

type preparedMissionPlanning struct {
	request   MissionPlanningRequest
	record    agentstate.MissionRecord
	context   *preparedPlanInvestigation
	store     *agentstate.Store
	inputHash string
}

// PlanMission is an explicit generation request. Its own bounded planning run
// produces a fresh unapproved draft; existing plans remain untouched on failure.
func (s *Service) PlanMission(ctx context.Context, request MissionPlanningRequest) (*MissionPlanningResult, error) {
	if err := s.RequireRemoteConfirmation(config.CodeModelScope, request.ConfirmRemoteProvider); err != nil {
		return nil, err
	}
	if agentruntime.BudgetFromContext(ctx) != nil {
		return nil, fmt.Errorf("mission planning requires its own bounded request")
	}
	frozen, err := s.freezeAgentSettings(ctx, s.manager.Root())
	if err != nil {
		return nil, err
	}
	prepared, err := s.prepareMissionPlanning(frozen, request)
	if err != nil {
		return nil, err
	}
	runtime := s.runtimes.function
	timed, stop := context.WithTimeout(frozen, duration(runtime.effective.Timeout))
	defer stop()
	bounded, release, err := agentruntime.WithBudget(timed, config.AgentLimits(request.PlanningLimits))
	if err != nil {
		return nil, err
	}
	defer release()
	authorize := func(ctx context.Context) error { return s.validateMissionPlanning(ctx, prepared) }
	bounded = agentruntime.WithContextAuthority(bounded, authorize)
	input, err := s.missionPlanningInput(bounded, prepared, runtime, authorize)
	if err != nil {
		return nil, err
	}
	output, err := s.retryAgentRequest(bounded, runtime, input, authorize)
	if err != nil {
		return nil, err
	}
	plan, err := agentruntime.ParseMissionPlan(output, prepared.plannerInput())
	if err != nil {
		return nil, err
	}
	prepared.record.Plan = plan
	return s.publishMissionPlan(bounded, prepared, input)
}

func (s *Service) prepareMissionPlanning(ctx context.Context, request MissionPlanningRequest) (*preparedMissionPlanning, error) {
	request.Sources = slices.Clone(request.Sources)
	if err := validateMissionPlanningRequest(request, s.admittedRuntime(ctx).Limits); err != nil {
		return nil, err
	}
	root := s.manager.Root()
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	store, catalog, err := s.missionCatalog(ctx, query, root)
	if err != nil {
		return nil, err
	}
	if catalog.Revision != request.CatalogRevision {
		return nil, ErrMissionConflict
	}
	if len(catalog.Missions) >= agentstate.MaxMissions {
		return nil, fmt.Errorf("mission catalog is full; planning cannot publish another draft")
	}
	captured, err := s.captureMissionSources(ctx, root, query, request.Sources)
	if err != nil {
		return nil, err
	}
	record := agentstate.MissionRecord{ID: "mission-" + rand.Text(), ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Policy: captured.policy.Version(), Instructions: map[string]string{}, Plan: agentstate.MissionPlan{Goal: request.Goal, Sources: request.Sources, Limits: request.Limits}}
	for path, instructions := range captured.instructions {
		record.Instructions[path] = instructions.Fingerprint
	}
	record.Configuration, err = s.agentConfiguration(s.admittedRuntime(ctx))
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal([]any{agentruntime.MissionPlannerVersion, request, record.Policy, record.Configuration, record.Instructions})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	p := &preparedMissionPlanning{request: request, record: record, context: captured, store: store, inputHash: hex.EncodeToString(digest[:])}
	return p, s.validateMissionPlanning(ctx, p)
}

func validateMissionPlanningRequest(request MissionPlanningRequest, ceiling config.AgentLimits) error {
	if request.ProjectID == "" || request.ProjectRevision == "" || request.CatalogRevision < 0 || request.CatalogRevision >= 1<<53 {
		return fmt.Errorf("provide current project and mission catalog identities")
	}
	if !validInvestigationText(request.Goal, 4096) || strings.IndexFunc(request.Goal, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) >= 0 {
		return fmt.Errorf("mission goal must contain 1–4096 UTF-8 bytes")
	}
	if err := request.Limits.Within(ceiling); err != nil {
		return err
	}
	if err := validateMissionPlanningLimits(request.PlanningLimits, ceiling); err != nil {
		return err
	}
	return validateMissionInventory(request.Sources)
}

func validateMissionPlanningLimits(limits agentstate.MissionLimits, ceiling config.AgentLimits) error {
	if err := limits.Within(ceiling); err != nil {
		return err
	}
	if limits.ModelCalls > 8 || limits.ToolCalls > 24 || limits.InputTokens > 1<<20 || limits.OutputTokens > 8<<20 || limits.ElapsedSeconds > 300 || limits.ConcurrentCalls != 1 || limits.Repairs != 0 {
		return fmt.Errorf("planning permits at most eight model calls, 24 tools, 1 Mi input tokens, 8 Mi output tokens and 300 seconds, with concurrency one and no repair loop")
	}
	return nil
}

func validateMissionInventory(sources []agentstate.MissionSource) error {
	if len(sources) == 0 || len(sources) > agentstate.MaxMissionSources {
		return fmt.Errorf("mission planning requires 1–64 captured paths")
	}
	for i, source := range sources {
		if !utf8.ValidString(source.Path) || source.Exists != (source.Hash != "") {
			return fmt.Errorf("invalid mission source identity")
		}
		if source.Exists && (agentstate.MemorySource{Path: source.Path, Hash: source.Hash}).Validate() != nil {
			return fmt.Errorf("invalid captured source hash")
		}
		for _, previous := range sources[:i] {
			if previous.Path == source.Path || strings.HasPrefix(previous.Path, source.Path+"/") || strings.HasPrefix(source.Path, previous.Path+"/") {
				return fmt.Errorf("mission inventory requires distinct file paths")
			}
		}
	}
	return nil
}

func (p *preparedMissionPlanning) plannerInput() agentruntime.MissionPlannerInput {
	return agentruntime.MissionPlannerInput{Goal: p.request.Goal, Sources: p.request.Sources, Limits: p.request.Limits}
}

func (s *Service) validateMissionPlanning(ctx context.Context, p *preparedMissionPlanning) error {
	if err := s.RequireRemoteConfirmation(config.CodeModelScope, p.request.ConfirmRemoteProvider); err != nil {
		return err
	}
	if err := s.validateMissionBinding(ctx, p.context.root, p.record); err != nil {
		return err
	}
	catalog, err := p.store.Missions(ctx, p.request.ProjectID)
	if context.Cause(ctx) != nil {
		return context.Cause(ctx)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMissionUnavailable, err)
	}
	if catalog.Revision != p.request.CatalogRevision {
		return ErrMissionConflict
	}
	return context.Cause(ctx)
}

func (s *Service) missionPlanningInput(ctx context.Context, p *preparedMissionPlanning, runtime modelRuntime, authorize func(context.Context) error) (agentruntime.Request, error) {
	tools := llm.ProviderCapabilities(runtime.profile).Tools && len(p.context.scopes) > 0
	var investigation *agentruntime.Investigation
	if tools {
		reader, err := project.NewAgentContext(p.context.index, p.context.scopes, p.context.policy.ReadSource)
		if err != nil {
			return agentruntime.Request{}, err
		}
		investigation, err = agentruntime.NewInvestigation(p.inputHash, repositoryTools(reader), authorize)
		if err != nil {
			return agentruntime.Request{}, err
		}
	}
	input, err := agentruntime.MissionPlannerRequest(p.plannerInput(), p.planningContext(tools), investigation, declarationContextLimit(runtime.effective.ContextMaxTokens))
	if err != nil {
		return input, err
	}
	p.context.manifest.EstimatedTokens = (len(input.Messages[0].Content) + 3) / 4
	p.context.manifest.TokenLimit = input.ContextMaxTokens
	return input, context.Cause(ctx)
}

func (p *preparedMissionPlanning) planningContext(tools bool) string {
	var text strings.Builder
	seen := map[string]bool{}
	for _, source := range p.request.Sources {
		for _, guide := range p.context.instructions[source.Path].Files {
			if !seen[guide.Path] {
				text.WriteString("\n" + project.InstructionPromptGuidance + "\n" + guide.Path + "\n" + guide.Content)
				seen[guide.Path] = true
			}
		}
		if !tools && source.Exists {
			text.WriteString("\nCaptured source " + source.Path + " (" + source.Hash + "):\n" + p.context.sources[source.Path])
		}
	}
	return text.String()
}

func (s *Service) publishMissionPlan(ctx context.Context, p *preparedMissionPlanning, input agentruntime.Request) (*MissionPlanningResult, error) {
	authorize := func() error {
		if err := s.validateMissionPlanning(ctx, p); err != nil {
			return err
		}
		return input.Investigation.Validate(ctx)
	}
	snapshot, err := p.store.SaveMission(ctx, p.record, p.request.CatalogRevision, "", authorize)
	if errors.Is(err, agentstate.ErrCorrupt) || errors.Is(err, agentstate.ErrPersistence) {
		err = fmt.Errorf("%w: %w", ErrMissionUnavailable, err)
	}
	if err != nil {
		return nil, err
	}
	return &MissionPlanningResult{AgentMission: AgentMission{MissionSnapshot: snapshot, SourceStatus: "current"}, InputHash: p.inputHash, PromptVersion: agentruntime.MissionPlannerVersion, PlanningLimits: p.request.PlanningLimits, Usage: agentruntime.BudgetFromContext(ctx).Snapshot(), ContextManifest: s.contextManifestForRuntime(p.context.manifest, s.runtimes.function)}, nil
}
