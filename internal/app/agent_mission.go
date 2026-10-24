package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

var ErrMissionNotFound = agentstate.ErrNotFound
var ErrMissionConflict = agentstate.ErrMissionConflict
var ErrMissionUnavailable = errors.New("private mission catalog unavailable")

type MissionMutation struct {
	ProjectID       string                  `json:"project_id"`
	ProjectRevision string                  `json:"project_revision"`
	CatalogRevision int64                   `json:"catalog_revision"`
	PlanHash        string                  `json:"plan_hash"`
	Action          string                  `json:"action"`
	ChildID         string                  `json:"child_id,omitempty"`
	Confirm         bool                    `json:"confirm"`
	Plan            *agentstate.MissionPlan `json:"plan,omitempty"`
}

func (request *MissionMutation) UnmarshalJSON(data []byte) error {
	type plain MissionMutation
	var value plain
	if err := decodeMissionJSON(data, &value); err != nil {
		return err
	}
	*request = MissionMutation(value)
	return nil
}

func decodeMissionJSON(data []byte, destination any) error {
	if !jsontext.Value(data).IsValid() || !bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		return fmt.Errorf("mission request must be one object with distinct field names")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode mission request: %w", err)
	}
	return nil
}

type AgentMission struct {
	agentstate.MissionSnapshot
	Stale        bool   `json:"stale"`
	SourceStatus string `json:"source_status"`
}

type MissionSummary struct {
	ID              string    `json:"id"`
	ProjectRevision string    `json:"project_revision"`
	Goal            string    `json:"goal"`
	PlanHash        string    `json:"plan_hash"`
	ChildCount      int       `json:"child_count"`
	ReviewedCount   int       `json:"reviewed_count"`
	Approved        bool      `json:"approved"`
	Stale           bool      `json:"stale"`
	SourceStatus    string    `json:"source_status"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type MissionList struct {
	ProjectID       string           `json:"project_id"`
	ProjectRevision string           `json:"project_revision"`
	CatalogRevision int64            `json:"catalog_revision"`
	Missions        []MissionSummary `json:"missions"`
}

func (s *Service) missionCatalog(ctx context.Context, query AgentRunQuery, root string) (*agentstate.Store, agentstate.MissionCatalog, error) {
	if err := s.agentHistoryScope(query, root); err != nil {
		return nil, agentstate.MissionCatalog{}, err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, agentstate.MissionCatalog{}, fmt.Errorf("%w: %w", ErrMissionUnavailable, err)
	}
	catalog, err := store.Missions(ctx, query.ProjectID)
	if err != nil && context.Cause(ctx) == nil {
		err = fmt.Errorf("%w: %w", ErrMissionUnavailable, err)
	}
	return store, catalog, err
}

func (s *Service) ListMissions(ctx context.Context, query AgentRunQuery) (*MissionList, error) {
	root := s.manager.Root()
	_, catalog, err := s.missionCatalog(ctx, query, root)
	if err != nil {
		return nil, err
	}
	result := &MissionList{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, CatalogRevision: catalog.Revision, Missions: []MissionSummary{}}
	for _, record := range catalog.Missions {
		result.Missions = append(result.Missions, MissionSummary{ID: record.ID, ProjectRevision: record.ProjectRevision, Goal: record.Plan.Goal, PlanHash: record.PlanHash, ChildCount: len(record.Plan.Children), ReviewedCount: len(record.Reviewed), Approved: record.Approved, Stale: record.ProjectRevision != query.ProjectRevision, SourceStatus: "unchecked", UpdatedAt: record.UpdatedAt})
	}
	return result, s.agentHistoryScope(query, root)
}

func findMission(catalog agentstate.MissionCatalog, id string) (agentstate.MissionRecord, error) {
	for _, record := range catalog.Missions {
		if record.ID == id {
			return record, nil
		}
	}
	return agentstate.MissionRecord{}, ErrMissionNotFound
}

func (s *Service) MissionDetail(ctx context.Context, query AgentRunQuery, id string) (*AgentMission, error) {
	root := s.manager.Root()
	_, catalog, err := s.missionCatalog(ctx, query, root)
	if err != nil {
		return nil, err
	}
	record, err := findMission(catalog, id)
	if err != nil {
		return nil, err
	}
	result := &AgentMission{MissionSnapshot: agentstate.MissionSnapshot{CatalogRevision: catalog.Revision, Mission: record}, SourceStatus: "current"}
	if err := s.validateMissionBinding(ctx, root, record); err != nil {
		if context.Cause(ctx) != nil {
			return nil, context.Cause(ctx)
		}
		result.Stale, result.SourceStatus = true, "unavailable"
		if memorySourceChanged(err) {
			result.SourceStatus = "changed"
		}
	}
	return result, s.agentHistoryScope(query, root)
}

// MutateMission stores private planning decisions only. It has no provider,
// subprocess, worktree, change-session, Apply or Undo dispatch path.
func (s *Service) MutateMission(ctx context.Context, id string, request MissionMutation) (*AgentMission, error) {
	if err := validateMissionMutation(id, request); err != nil {
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
	record, err := s.missionMutationRecord(ctx, root, id, request, catalog)
	if err != nil {
		return nil, err
	}
	authorize := func() error { return s.validateMissionBinding(ctx, root, record) }
	var snapshot agentstate.MissionSnapshot
	if request.Plan != nil {
		snapshot, err = store.SaveMission(ctx, record, request.CatalogRevision, request.PlanHash, authorize)
	} else {
		snapshot, err = store.ReviewMission(ctx, query.ProjectID, id, request.CatalogRevision, request.PlanHash, request.ChildID, authorize)
	}
	if errors.Is(err, agentstate.ErrPersistence) || errors.Is(err, agentstate.ErrCorrupt) {
		err = fmt.Errorf("%w: %w", ErrMissionUnavailable, err)
	}
	if err != nil {
		return nil, err
	}
	return &AgentMission{MissionSnapshot: snapshot, SourceStatus: "current"}, nil
}

func validateMissionMutation(id string, request MissionMutation) error {
	if request.ProjectID == "" || request.ProjectRevision == "" || request.CatalogRevision < 0 || request.CatalogRevision >= 1<<53 {
		return fmt.Errorf("provide project identity and a current catalog revision")
	}
	if !slices.Contains([]string{"create", "revise", "review", "approve"}, request.Action) {
		return fmt.Errorf("unknown mission action")
	}
	if request.Action == "create" {
		if id != "" || request.PlanHash != "" {
			return fmt.Errorf("create requires no mission identity")
		}
	} else if id == "" || request.PlanHash == "" || request.CatalogRevision == 0 {
		return fmt.Errorf("provide the saved mission hash and catalog revision")
	}
	return validateMissionAction(request)
}

func validateMissionAction(request MissionMutation) error {
	if (request.Action == "create" || request.Action == "revise") != (request.Plan != nil) {
		return fmt.Errorf("only create and revise require a complete plan")
	}
	if (request.Action == "review") != (request.ChildID != "") {
		return fmt.Errorf("only child review requires a child identity")
	}
	if request.Plan == nil && !request.Confirm {
		return fmt.Errorf("explicitly confirm the child review or mission approval")
	}
	return nil
}

func (s *Service) missionMutationRecord(ctx context.Context, root, id string, request MissionMutation, catalog agentstate.MissionCatalog) (agentstate.MissionRecord, error) {
	var previous agentstate.MissionRecord
	if id != "" {
		var err error
		previous, err = findMission(catalog, id)
		if err != nil {
			return previous, err
		}
		if previous.PlanHash != request.PlanHash {
			return previous, ErrMissionConflict
		}
		if err := s.validateMissionBinding(ctx, root, previous); err != nil {
			return previous, err
		}
	}
	if request.Plan == nil {
		return previous, nil
	}
	record, _, err := s.prepareMission(ctx, root, AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}, *request.Plan)
	if err != nil {
		return record, err
	}
	record.ID = id
	if record.ID == "" {
		record.ID = "mission-" + rand.Text()
	}
	return record, nil
}

func (s *Service) prepareMission(ctx context.Context, root string, query AgentRunQuery, plan agentstate.MissionPlan) (agentstate.MissionRecord, *preparedPlanInvestigation, error) {
	record := agentstate.MissionRecord{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Plan: plan, Instructions: map[string]string{}}
	if err := plan.Validate(); err != nil {
		return record, nil, err
	}
	if err := s.agentHistoryScope(query, root); err != nil {
		return record, nil, err
	}
	frozen, err := s.freezeAgentSettings(ctx, root)
	if err != nil {
		return record, nil, err
	}
	runtime := s.admittedRuntime(frozen)
	if err := plan.Limits.Within(runtime.Limits); err != nil {
		return record, nil, err
	}
	record.Configuration, err = s.agentConfiguration(runtime)
	if err != nil {
		return record, nil, err
	}
	prepared, err := s.captureMissionSources(ctx, root, query, plan.Sources)
	if err != nil {
		return record, nil, err
	}
	record.Policy = prepared.policy.Version()
	for path, instructions := range prepared.instructions {
		record.Instructions[path] = instructions.Fingerprint
	}
	return record, prepared, s.validateMissionBinding(ctx, root, record)
}

func (s *Service) captureMissionSources(ctx context.Context, root string, query AgentRunQuery, sources []agentstate.MissionSource) (*preparedPlanInvestigation, error) {
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	p := &preparedPlanInvestigation{root: root, index: index, policy: policy, request: PlanInvestigationRequest{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision}, sources: map[string]string{}, instructions: map[string]project.EffectiveInstructions{}, manifest: project.ContextManifest{Included: []project.ContextFile{}, Excluded: []project.ContextDecision{}, ByteLimit: maxChangeBytes}}
	for _, source := range sources {
		captured := PlanInvestigationSource{Path: source.Path, Hash: source.Hash, Exists: source.Exists}
		p.request.Sources = append(p.request.Sources, captured)
		if err := p.capture(ctx, captured); err != nil {
			return nil, err
		}
	}
	return p, s.validatePlanInvestigation(ctx, p)
}

func (s *Service) validateMissionBinding(ctx context.Context, root string, record agentstate.MissionRecord) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	query := AgentRunQuery{ProjectID: record.ProjectID, ProjectRevision: record.ProjectRevision}
	if err := s.agentHistoryScope(query, root); err != nil {
		return err
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return err
	}
	if policy.Version() != record.Policy {
		return project.ErrRevisionConflict
	}
	p := &preparedPlanInvestigation{root: root, instructions: map[string]project.EffectiveInstructions{}}
	for _, source := range record.Plan.Sources {
		p.instructions[source.Path] = project.EffectiveInstructions{Fingerprint: record.Instructions[source.Path]}
		captured := PlanInvestigationSource{Path: source.Path, Hash: source.Hash, Exists: source.Exists}
		if !policy.Decide(source.Path).Include {
			return project.ErrExcludedFile
		}
		if err := p.validateSource(ctx, policy, captured); err != nil {
			return err
		}
	}
	if err := s.validateMissionConfiguration(ctx, root, record.Configuration); err != nil {
		return err
	}
	return s.agentHistoryScope(query, root)
}

func (s *Service) validateMissionConfiguration(ctx context.Context, root, expected string) error {
	// Re-read future-run settings instead of accidentally validating a frozen
	// admission against itself after a concurrent preference replacement.
	fresh, err := s.freezeAgentSettings(context.WithValue(ctx, agentSettingsKey{}, nil), root)
	if err != nil {
		return err
	}
	configuration, err := s.agentConfiguration(s.admittedRuntime(fresh))
	if err != nil {
		return err
	}
	if configuration != expected {
		return fmt.Errorf("%w: mission settings changed", project.ErrRevisionConflict)
	}
	return nil
}
