package agentstate

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// MissionBase is the reviewed integration checkout identity. It is evidence,
// not authority to run Git or to merge any child.
type MissionBase struct {
	Head   string `json:"head"`
	Branch string `json:"branch"`
}

type MissionExecutionChild struct {
	ID     string        `json:"id"`
	State  string        `json:"state"`
	Limits MissionLimits `json:"limits"`
}

// One execution belongs to one mission. Its frozen plan and fixed allocations
// survive cancellation and catalog edits; replay cannot mint another budget.
type MissionExecution struct {
	ID              string                  `json:"id"`
	Revision        int64                   `json:"execution_revision"`
	CatalogRevision int64                   `json:"catalog_revision"`
	Mission         MissionRecord           `json:"mission"`
	Base            MissionBase             `json:"base"`
	State           string                  `json:"state"`
	Children        []MissionExecutionChild `json:"children"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

type MissionExecutionPrepare struct {
	ProjectID       string
	MissionID       string
	CatalogRevision int64
	PlanHash        string
	Base            MissionBase
}

type MissionExecutionIdentity struct {
	ProjectID   string
	MissionID   string
	ExecutionID string
	Revision    int64
	PlanHash    string
}

type missionExecutions struct {
	Version   int                         `json:"version"`
	Workspace string                      `json:"workspace"`
	Records   map[string]MissionExecution `json:"records"`
}

func (record MissionExecution) validate() error {
	if !validID(record.ID) || record.Revision < 1 || record.Revision > 1<<53 || record.CatalogRevision < 1 || record.CatalogRevision > 1<<53 {
		return ErrCorrupt
	}
	if record.Mission.validate() != nil || !record.Mission.Approved || record.CreatedAt.IsZero() || record.UpdatedAt.Before(record.CreatedAt) {
		return ErrCorrupt
	}
	if record.State != "prepared" && record.State != "canceled" {
		return ErrCorrupt
	}
	if !validMissionBase(record.Base) {
		return ErrCorrupt
	}
	return record.validateAllocations()
}

func (record MissionExecution) validateAllocations() error {
	if len(record.Children) != len(record.Mission.Plan.Children) {
		return ErrCorrupt
	}
	for i, child := range record.Children {
		approved := record.Mission.Plan.Children[i]
		if child.ID != approved.ID || child.Limits != approved.Limits || child.State != "pending" {
			return ErrCorrupt
		}
	}
	return nil
}

func validMissionBase(base MissionBase) bool {
	if len(base.Head) != 40 && len(base.Head) != 64 {
		return false
	}
	_, err := hex.DecodeString(base.Head)
	return err == nil && validMemoryText(base.Branch, 240, false)
}

func (s *Store) loadMissionExecutions() (*missionExecutions, error) {
	location, err := s.path("mission-executions.json")
	if err != nil {
		return nil, fmt.Errorf("%w: locate mission executions: %w", ErrPersistence, err)
	}
	data, err := readJournalFile(location)
	if errors.Is(err, os.ErrNotExist) {
		return &missionExecutions{Version: 1, Workspace: s.workspace, Records: map[string]MissionExecution{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: read mission executions: %w", ErrPersistence, err)
	}
	var catalog missionExecutions
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if len(data) > maxMissionCatalog || !validJSON(data) || decoder.Decode(&catalog) != nil || s.validateMissionExecutions(&catalog) != nil {
		return nil, ErrCorrupt
	}
	return &catalog, nil
}

func (s *Store) validateMissionExecutions(catalog *missionExecutions) error {
	if catalog.Version != 1 || catalog.Workspace != s.workspace || catalog.Records == nil || len(catalog.Records) > MaxMissions {
		return ErrCorrupt
	}
	seen := map[string]bool{}
	for id, record := range catalog.Records {
		if id != record.Mission.ID || record.validate() != nil || seen[record.ID] {
			return ErrCorrupt
		}
		seen[record.ID] = true
	}
	return nil
}

func (s *Store) MissionExecution(ctx context.Context, projectID, missionID string) (MissionExecution, error) {
	if err := context.Cause(ctx); err != nil {
		return MissionExecution{}, err
	}
	if !validID(projectID) || !validID(missionID) {
		return MissionExecution{}, fmt.Errorf("mission execution requires project and mission identities")
	}
	catalog, err := s.loadMissionExecutions()
	if err != nil {
		return MissionExecution{}, err
	}
	record, exists := catalog.Records[missionID]
	if !exists || record.Mission.ProjectID != projectID {
		return MissionExecution{}, ErrNotFound
	}
	return record, context.Cause(ctx)
}

func (s *Store) mutateMissionExecutions(ctx context.Context, authorize func() error, change func(*missionExecutions, time.Time) error) (err error) {
	if err := authorizeMissionWrite(ctx, authorize); err != nil {
		return err
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	lock, err := s.lock(ctx)
	if err != nil {
		return persistenceOperationError(parent, "lock mission executions", err)
	}
	defer func() {
		if closeErr := lock.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("%w: release mission execution lock: %w", ErrPersistence, closeErr))
		}
	}()
	catalog, err := s.loadMissionExecutions()
	if err != nil {
		return err
	}
	if err := change(catalog, time.Now().UTC()); err != nil {
		return err
	}
	if err := s.validateMissionExecutions(catalog); err != nil {
		return err
	}
	data, err := json.Marshal(catalog)
	if err != nil {
		return fmt.Errorf("encode mission executions: %w", err)
	}
	if len(data) > maxMissionCatalog {
		return fmt.Errorf("mission execution catalog exceeds its 8 MiB limit")
	}
	location, err := s.path("mission-executions.json")
	if err != nil {
		return err
	}
	if err := s.write(location, data, 0600, func() error { return authorizeMissionWrite(ctx, authorize) }); err != nil {
		return persistenceOperationError(parent, "save mission executions", err)
	}
	return nil
}

func (s *Store) PrepareMissionExecution(ctx context.Context, request MissionExecutionPrepare, authorize func() error) (result MissionExecution, err error) {
	err = s.mutateMissionExecutions(ctx, authorize, func(executions *missionExecutions, now time.Time) error {
		catalog, err := s.loadMissions()
		if err != nil {
			return err
		}
		mission, exists := catalog.Records[request.MissionID]
		if !exists || mission.ProjectID != request.ProjectID {
			return ErrNotFound
		}
		if catalog.Revision != request.CatalogRevision || mission.PlanHash != request.PlanHash {
			return ErrMissionConflict
		}
		if _, exists := executions.Records[mission.ID]; exists {
			return fmt.Errorf("%w: mission already owns an execution; retain it and create a new mission for another attempt", ErrMissionConflict)
		}
		if !mission.Approved {
			return fmt.Errorf("review every child and approve the mission before preparing execution")
		}
		result = MissionExecution{ID: "mission-run-" + rand.Text(), Revision: 1, CatalogRevision: catalog.Revision, Mission: mission, Base: request.Base, State: "prepared", CreatedAt: now, UpdatedAt: now}
		for _, child := range mission.Plan.Children {
			result.Children = append(result.Children, MissionExecutionChild{ID: child.ID, State: "pending", Limits: child.Limits})
		}
		executions.Records[mission.ID] = result
		return nil
	})
	if err != nil {
		return MissionExecution{}, err
	}
	return result, nil
}

func (s *Store) CancelPreparedMissionExecution(ctx context.Context, request MissionExecutionIdentity, authorize func() error) (result MissionExecution, err error) {
	err = s.mutateMissionExecutions(ctx, authorize, func(catalog *missionExecutions, now time.Time) error {
		record, exists := catalog.Records[request.MissionID]
		if !exists || record.Mission.ProjectID != request.ProjectID {
			return ErrNotFound
		}
		if record.ID != request.ExecutionID || record.Revision != request.Revision || record.Mission.PlanHash != request.PlanHash || record.State != "prepared" {
			return ErrMissionConflict
		}
		record.State, record.UpdatedAt = "canceled", now
		record.Revision++
		catalog.Records[request.MissionID] = record
		result = record
		return nil
	})
	if err != nil {
		return MissionExecution{}, err
	}
	return result, nil
}
