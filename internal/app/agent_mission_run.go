package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type MissionExecutionRequest struct {
	ProjectID         string                  `json:"project_id"`
	ProjectRevision   string                  `json:"project_revision"`
	Action            string                  `json:"action"`
	CatalogRevision   int64                   `json:"catalog_revision"`
	PlanHash          string                  `json:"plan_hash"`
	ExecutionID       string                  `json:"execution_id,omitempty"`
	ExecutionRevision int64                   `json:"execution_revision"`
	Base              *agentstate.MissionBase `json:"base,omitempty"`
	Confirm           bool                    `json:"confirm"`
}

func (request *MissionExecutionRequest) UnmarshalJSON(data []byte) error {
	type plain MissionExecutionRequest
	var value plain
	if err := decodeMissionJSON(data, &value); err != nil {
		return err
	}
	*request = MissionExecutionRequest(value)
	return nil
}

type AgentMissionExecution struct {
	Execution    agentstate.MissionExecution `json:"execution"`
	Stale        bool                        `json:"stale"`
	SourceStatus string                      `json:"source_status"`
}

func validateMissionExecutionRequest(id string, request MissionExecutionRequest) error {
	if id == "" || !request.Confirm || request.ProjectID == "" || request.ProjectRevision == "" || len(request.PlanHash) != 64 {
		return fmt.Errorf("confirm the exact mission plan and current project before preparing or canceling execution")
	}
	return validateMissionExecutionAction(request)
}

func validateMissionExecutionAction(request MissionExecutionRequest) error {
	switch request.Action {
	case "prepare":
		if request.CatalogRevision < 1 || request.ExecutionID != "" || request.ExecutionRevision != 0 || request.Base == nil {
			return fmt.Errorf("preparation requires the approved catalog revision and Git base, without a previous execution identity")
		}
	case "cancel":
		if request.ExecutionID == "" || request.ExecutionRevision < 1 || request.CatalogRevision != 0 || request.Base != nil {
			return fmt.Errorf("cancellation requires the exact execution identity and revision, without a new catalog or Git base")
		}
	default:
		return fmt.Errorf("choose prepare or cancel for a mission execution")
	}
	return nil
}

func missionExecutionError(err error) error {
	if errors.Is(err, agentstate.ErrPersistence) || errors.Is(err, agentstate.ErrCorrupt) {
		return fmt.Errorf("%w: %w", ErrMissionUnavailable, err)
	}
	return err
}

func (s *Service) missionExecutionStore(query AgentRunQuery, root string) (*agentstate.Store, error) {
	if err := s.agentHistoryScope(query, root); err != nil {
		return nil, err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMissionUnavailable, err)
	}
	return store, nil
}

func (s *Service) MissionExecution(ctx context.Context, query AgentRunQuery, id string) (*AgentMissionExecution, error) {
	root := s.manager.Root()
	store, err := s.missionExecutionStore(query, root)
	if err != nil {
		return nil, err
	}
	record, err := store.MissionExecution(ctx, query.ProjectID, id)
	if err != nil {
		return nil, missionExecutionError(err)
	}
	return s.inspectMissionExecution(ctx, root, query, record)
}

// ControlMissionExecution prepares or cancels a private receipt. Neither action
// grants provider, project execution, worktree, Apply or merge authority.
func (s *Service) ControlMissionExecution(ctx context.Context, id string, request MissionExecutionRequest) (*AgentMissionExecution, error) {
	if err := validateMissionExecutionRequest(id, request); err != nil {
		return nil, err
	}
	root := s.manager.Root()
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	store, err := s.missionExecutionStore(query, root)
	if err != nil {
		return nil, err
	}
	var record agentstate.MissionExecution
	if request.Action == "prepare" {
		record, err = s.prepareMissionExecution(ctx, root, store, id, request)
	} else {
		identity := agentstate.MissionExecutionIdentity{ProjectID: query.ProjectID, MissionID: id, ExecutionID: request.ExecutionID, Revision: request.ExecutionRevision, PlanHash: request.PlanHash}
		record, err = store.CancelPreparedMissionExecution(ctx, identity, func() error { return s.agentHistoryScope(query, root) })
	}
	if err != nil {
		return nil, missionExecutionError(err)
	}
	return s.inspectMissionExecution(ctx, root, query, record)
}

func (s *Service) prepareMissionExecution(ctx context.Context, root string, store *agentstate.Store, id string, request MissionExecutionRequest) (agentstate.MissionExecution, error) {
	catalog, err := store.Missions(ctx, request.ProjectID)
	if err != nil {
		return agentstate.MissionExecution{}, err
	}
	mission, err := findMission(catalog, id)
	if err != nil {
		return agentstate.MissionExecution{}, err
	}
	if catalog.Revision != request.CatalogRevision || mission.PlanHash != request.PlanHash {
		return agentstate.MissionExecution{}, ErrMissionConflict
	}
	if !mission.Approved {
		return agentstate.MissionExecution{}, fmt.Errorf("review every child and approve the mission before preparing execution")
	}
	authorize := func() error {
		if err := s.validateMissionBinding(ctx, root, mission); err != nil {
			return err
		}
		return s.validateMissionBase(ctx, mission, *request.Base)
	}
	return store.PrepareMissionExecution(ctx, agentstate.MissionExecutionPrepare{ProjectID: request.ProjectID, MissionID: id, CatalogRevision: request.CatalogRevision, PlanHash: request.PlanHash, Base: *request.Base}, authorize)
}

func (s *Service) validateMissionBase(ctx context.Context, mission agentstate.MissionRecord, base agentstate.MissionBase) error {
	repository, err := s.GitRepository(ctx, mission.ProjectID, mission.ProjectRevision)
	if err != nil {
		return err
	}
	if !repository.Available || !repository.AtRoot || repository.Head == "" || repository.Branch == "" {
		return fmt.Errorf("mission execution requires a Git checkout root on a committed local branch")
	}
	if repository.Dirty || repository.Operation != "" || repository.MergeHead != "" || len(repository.Conflicts) != 0 {
		return fmt.Errorf("%w: finish repository operations and commit local changes before preparing mission execution", project.ErrRevisionConflict)
	}
	return requireMatchingGitIdentity(repository, GitIdentity{Head: base.Head, Branch: base.Branch})
}

func (s *Service) inspectMissionExecution(ctx context.Context, root string, query AgentRunQuery, record agentstate.MissionExecution) (*AgentMissionExecution, error) {
	result := &AgentMissionExecution{Execution: record, SourceStatus: "current"}
	if err := s.validatePreparedMissionExecution(ctx, root, record); err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return nil, cause
		}
		result.Stale, result.SourceStatus = true, "unavailable"
		if memorySourceChanged(err) || errors.Is(err, ErrMissionConflict) {
			result.SourceStatus = "changed"
		}
	}
	return result, s.agentHistoryScope(query, root)
}

func (s *Service) validatePreparedMissionExecution(ctx context.Context, root string, record agentstate.MissionExecution) error {
	if err := s.validateMissionBinding(ctx, root, record.Mission); err != nil {
		return err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return err
	}
	catalog, err := store.Missions(ctx, record.Mission.ProjectID)
	if err != nil {
		return err
	}
	current, err := findMission(catalog, record.Mission.ID)
	if err != nil {
		return err
	}
	if current.PlanHash != record.Mission.PlanHash || !current.Approved {
		return ErrMissionConflict
	}
	return s.validateMissionBase(ctx, record.Mission, record.Base)
}
