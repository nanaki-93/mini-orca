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
	"slices"
	"strings"
	"time"
)

// MissionBase is the reviewed integration checkout identity. It is evidence,
// not authority to run Git or to merge any child.
type MissionBase struct {
	Head   string `json:"head"`
	Branch string `json:"branch"`
}

type MissionExecutionChild struct {
	ID               string        `json:"id"`
	Generation       int           `json:"generation,omitempty"`
	State            string        `json:"state"`
	Limits           MissionLimits `json:"limits"`
	WaveID           string        `json:"wave_id,omitempty"`
	Directory        string        `json:"directory,omitempty"`
	Branch           string        `json:"branch,omitempty"`
	BaseHead         string        `json:"base_head,omitempty"`
	ProjectID        string        `json:"project_id,omitempty"`
	ProjectRevision  string        `json:"project_revision,omitempty"`
	SessionID        string        `json:"session_id,omitempty"`
	WorkflowID       string        `json:"workflow_id,omitempty"`
	ProposalHash     string        `json:"proposal_hash,omitempty"`
	ProposalRevision int64         `json:"proposal_revision,omitempty"`
	Reason           string        `json:"reason,omitempty"`
	UpdatedAt        time.Time     `json:"updated_at,omitzero"`
}

type MissionWave struct {
	ID              string    `json:"id"`
	Owner           string    `json:"owner"`
	Children        []string  `json:"children"`
	StartedAt       time.Time `json:"started_at"`
	AllowanceMillis int64     `json:"allowance_millis"`
}

func (wave MissionWave) Deadline() time.Time {
	return wave.StartedAt.Add(time.Duration(wave.AllowanceMillis) * time.Millisecond)
}

// One execution belongs to one mission. Its frozen plan and fixed allocations
// survive cancellation and catalog edits; replay cannot mint another budget.
type MissionExecution struct {
	ID              string                  `json:"id"`
	Revision        int64                   `json:"execution_revision"`
	CatalogRevision int64                   `json:"catalog_revision"`
	Mission         MissionRecord           `json:"mission"`
	Base            MissionBase             `json:"base"`
	Current         MissionCheckpoint       `json:"current"`
	Integrations    []MissionIntegration    `json:"integrations,omitempty"`
	Revalidations   []MissionRevalidation   `json:"revalidations,omitempty"`
	State           string                  `json:"state"`
	ElapsedMillis   int64                   `json:"elapsed_millis,omitempty"`
	Wave            *MissionWave            `json:"wave,omitempty"`
	Failure         string                  `json:"failure,omitempty"`
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
	if err := record.validateHeader(); err != nil {
		return err
	}
	if err := record.validateAllocations(); err != nil {
		return err
	}
	if err := record.validateIntegrations(); err != nil {
		return err
	}
	if err := record.validateRevalidations(); err != nil {
		return err
	}
	return record.validateWave()
}

func (record MissionExecution) validateHeader() error {
	if !validID(record.ID) || record.Revision < 1 || record.Revision > 1<<53 || record.CatalogRevision < 1 || record.CatalogRevision > 1<<53 {
		return ErrCorrupt
	}
	if record.Mission.validate() != nil || !record.Mission.Approved || record.CreatedAt.IsZero() || record.UpdatedAt.Before(record.CreatedAt) {
		return ErrCorrupt
	}
	if !validMissionBase(record.Base) {
		return ErrCorrupt
	}
	if record.Failure != "" && !validMemoryText(record.Failure, 1024, true) {
		return ErrCorrupt
	}
	return nil
}

func (record MissionExecution) validateAllocations() error {
	if len(record.Children) != len(record.Mission.Plan.Children) {
		return ErrCorrupt
	}
	for i, child := range record.Children {
		approved := record.Mission.Plan.Children[i]
		if child.ID != approved.ID || child.Limits != approved.Limits || child.validate(record.ID) != nil {
			return ErrCorrupt
		}
	}
	return nil
}

func (child MissionExecutionChild) validate(executionID string) error {
	if child.State == "pending" {
		if child != (MissionExecutionChild{ID: child.ID, State: "pending", Limits: child.Limits}) {
			return ErrCorrupt
		}
		return nil
	}
	if !slices.Contains([]string{"reserved", "running", "awaiting_review", "rejected", "failed", "interrupted", "integrated"}, child.State) || !validID(child.WaveID) || child.UpdatedAt.IsZero() {
		return ErrCorrupt
	}
	if child.Generation < 0 || child.Generation > MaxMissionRevalidations || child.Directory != missionCandidateDirectory(executionID, child.ID, child.Generation) || strings.ContainsAny(child.Directory, "/\\") || child.Branch != "codex/"+child.Directory || !validMissionBase(MissionBase{Head: child.BaseHead, Branch: child.Branch}) {
		return ErrCorrupt
	}
	if !validMemoryText(child.Reason, 1024, true) && child.Reason != "" {
		return ErrCorrupt
	}
	return child.validateProposal()
}

func (child MissionExecutionChild) validateProposal() error {
	if child.SessionID == "" {
		return child.validateMissingProposal()
	}
	for _, value := range []string{child.ProjectID, child.ProjectRevision, child.SessionID, child.ProposalHash} {
		if !validID(value) {
			return ErrCorrupt
		}
	}
	if child.Generation == 0 && !validID(child.WorkflowID) || child.Generation > 0 && child.WorkflowID != "" {
		return ErrCorrupt
	}
	if child.ProposalRevision < 0 {
		return ErrCorrupt
	}
	return nil
}

func (child MissionExecutionChild) validateMissingProposal() error {
	if child.ProjectID != "" || child.ProjectRevision != "" || child.WorkflowID != "" || child.ProposalHash != "" || child.ProposalRevision != 0 {
		return ErrCorrupt
	}
	if slices.Contains([]string{"running", "awaiting_review", "rejected", "integrated"}, child.State) {
		return ErrCorrupt
	}
	return nil
}

func (record MissionExecution) validateWave() error {
	if !slices.Contains([]string{"prepared", "running", "pausing", "canceling", "paused", "canceled", "awaiting_review", "blocked", "interrupted", "integrating", "ready", "completed", "revalidating"}, record.State) {
		return ErrCorrupt
	}
	active := record.State == "running" || record.State == "pausing" || record.State == "canceling"
	if active != (record.Wave != nil) || record.ElapsedMillis < 0 || record.ElapsedMillis > int64(record.Mission.Plan.Limits.ElapsedSeconds)*1000 {
		return ErrCorrupt
	}
	if record.Wave == nil {
		return record.validateInactiveChildren()
	}
	wave := record.Wave
	if !validID(wave.ID) || !validID(wave.Owner) || wave.StartedAt.IsZero() || wave.AllowanceMillis < 1 || wave.AllowanceMillis+record.ElapsedMillis > int64(record.Mission.Plan.Limits.ElapsedSeconds)*1000 {
		return ErrCorrupt
	}
	return record.validateWaveChildren()
}

func (record MissionExecution) validateInactiveChildren() error {
	for _, child := range record.Children {
		if child.State == "running" || child.State == "reserved" || record.State == "prepared" && child.State != "pending" {
			return ErrCorrupt
		}
	}
	if record.State == "prepared" && (record.ElapsedMillis != 0 || record.Failure != "") {
		return ErrCorrupt
	}
	return nil
}

func (record MissionExecution) validateWaveChildren() error {
	if len(record.Wave.Children) == 0 || len(record.Wave.Children) > len(record.Children) {
		return ErrCorrupt
	}
	seen, slots := map[string]bool{}, 0
	for _, id := range record.Wave.Children {
		index := slices.IndexFunc(record.Children, func(child MissionExecutionChild) bool { return child.ID == id })
		if index < 0 || seen[id] || record.Children[index].WaveID != record.Wave.ID {
			return ErrCorrupt
		}
		seen[id] = true
		slots += record.Children[index].Limits.ConcurrentCalls
	}
	if slots > record.Mission.Plan.Limits.ConcurrentCalls {
		return ErrCorrupt
	}
	for _, child := range record.Children {
		if (child.State == "running" || child.State == "reserved" || child.WaveID == record.Wave.ID) && !seen[child.ID] {
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
		return &missionExecutions{Version: 4, Workspace: s.workspace, Records: map[string]MissionExecution{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: read mission executions: %w", ErrPersistence, err)
	}
	var catalog missionExecutions
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if len(data) > maxMissionCatalog || !validJSON(data) || decoder.Decode(&catalog) != nil || upgradeMissionExecutions(&catalog) != nil || upgradeMissionRevalidations(&catalog) != nil || s.validateMissionExecutions(&catalog) != nil {
		return nil, ErrCorrupt
	}
	return &catalog, nil
}

func upgradeMissionExecutions(catalog *missionExecutions) error {
	if catalog.Version != 1 {
		return upgradeMissionIntegrationBindings(catalog)
	}
	for _, record := range catalog.Records {
		if record.State != "prepared" && record.State != "canceled" || record.Wave != nil || record.ElapsedMillis != 0 || record.Failure != "" {
			return ErrCorrupt
		}
		for _, child := range record.Children {
			if child.State != "pending" {
				return ErrCorrupt
			}
		}
	}
	catalog.Version = 2
	return upgradeMissionIntegrationBindings(catalog)
}

func (s *Store) validateMissionExecutions(catalog *missionExecutions) error {
	if catalog.Version != 4 || catalog.Workspace != s.workspace || catalog.Records == nil || len(catalog.Records) > MaxMissions {
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
		result.Current = initialMissionCheckpoint(result)
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

func (s *Store) CancelMissionExecution(ctx context.Context, request MissionExecutionIdentity, authorize func() error) (MissionExecution, error) {
	return s.stopMissionExecution(ctx, request, "cancel", authorize)
}

func (s *Store) PauseMissionExecution(ctx context.Context, request MissionExecutionIdentity, authorize func() error) (MissionExecution, error) {
	return s.stopMissionExecution(ctx, request, "pause", authorize)
}

func (s *Store) stopMissionExecution(ctx context.Context, request MissionExecutionIdentity, action string, authorize func() error) (result MissionExecution, err error) {
	err = s.mutateMissionExecutions(ctx, authorize, func(catalog *missionExecutions, now time.Time) error {
		record, err := missionExecutionAt(catalog, request)
		if err != nil {
			return err
		}
		if record.State == "canceled" || record.State == "canceling" || action == "pause" && record.State != "running" {
			return ErrMissionConflict
		}
		record.State = "canceled"
		if record.Wave != nil {
			record.State = "canceling"
			if action == "pause" {
				record.State = "pausing"
			}
		}
		record.UpdatedAt = now
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

func missionExecutionAt(catalog *missionExecutions, request MissionExecutionIdentity) (MissionExecution, error) {
	record, exists := catalog.Records[request.MissionID]
	if !exists || record.Mission.ProjectID != request.ProjectID {
		return MissionExecution{}, ErrNotFound
	}
	if record.ID != request.ExecutionID || record.Revision != request.Revision || record.Mission.PlanHash != request.PlanHash {
		return MissionExecution{}, ErrMissionConflict
	}
	return record, nil
}

// ReadyChildren reserves whole child concurrency allocations. Since the plan
// requires dependency edges for overlapping reads/writes, this ready wave is
// disjoint even if completion order differs from integration order.
func (record MissionExecution) ReadyChildren() []string {
	ready := []string{}
	if record.Wave != nil || record.PendingIntegration() != nil || record.PendingRevalidation() != nil || record.State == "canceled" {
		return ready
	}
	states := map[string]string{}
	for _, child := range record.Children {
		states[child.ID] = child.State
	}
	slots := record.Mission.Plan.Limits.ConcurrentCalls
	for _, id := range record.Mission.Plan.IntegrationOrder {
		index := slices.IndexFunc(record.Mission.Plan.Children, func(child MissionChild) bool { return child.ID == id })
		child := record.Mission.Plan.Children[index]
		if states[id] != "pending" || child.Limits.ConcurrentCalls > slots {
			continue
		}
		if slices.ContainsFunc(child.Dependencies, func(id string) bool { return states[id] != "integrated" }) {
			continue
		}
		ready = append(ready, id)
		slots -= child.Limits.ConcurrentCalls
	}
	return ready
}

func (s *Store) BeginMissionWave(ctx context.Context, request MissionExecutionIdentity, owner string, authorize func() error) (result MissionExecution, err error) {
	err = s.mutateMissionExecutions(ctx, authorize, func(catalog *missionExecutions, now time.Time) error {
		record, err := missionExecutionAt(catalog, request)
		if err != nil {
			return err
		}
		if record.Wave != nil || record.PendingIntegration() != nil || record.PendingRevalidation() != nil || record.State == "canceled" {
			return ErrMissionConflict
		}
		ready := record.ReadyChildren()
		remaining := int64(record.Mission.Plan.Limits.ElapsedSeconds)*1000 - record.ElapsedMillis
		if len(ready) == 0 || remaining < 1 {
			return fmt.Errorf("no unreserved ready child or mission execution time remains; inspect dependencies and retained proposals")
		}
		wave := &MissionWave{ID: "wave-" + rand.Text(), Owner: owner, Children: ready, StartedAt: now}
		for i := range record.Children {
			child := &record.Children[i]
			if !slices.Contains(ready, child.ID) {
				continue
			}
			child.State, child.WaveID, child.UpdatedAt = "reserved", wave.ID, now
			child.Directory = record.ID + "-" + child.ID
			child.Branch, child.BaseHead = "codex/"+child.Directory, record.Current.Base.Head
			wave.AllowanceMillis = max(wave.AllowanceMillis, int64(child.Limits.ElapsedSeconds)*1000)
		}
		wave.AllowanceMillis = min(wave.AllowanceMillis, remaining)
		record.State, record.Wave, record.UpdatedAt = "running", wave, now
		record.Failure = ""
		record.Revision++
		catalog.Records[request.MissionID], result = record, record
		return nil
	})
	if err != nil {
		return MissionExecution{}, err
	}
	return result, nil
}

type MissionChildProgress struct {
	ProjectID, ProjectRevision, SessionID, WorkflowID, ProposalHash string
	ProposalRevision                                                int64
	State, Reason                                                   string
}

// Worker writes use the durable wave lease instead of a human editor revision;
// siblings may finish concurrently, but can never update a replacement wave.
func (s *Store) RecordMissionChild(ctx context.Context, identity MissionExecutionIdentity, waveID, childID string, progress MissionChildProgress, authorize func() error) (err error) {
	return s.mutateMissionExecutions(ctx, authorize, func(catalog *missionExecutions, now time.Time) error {
		record, err := missionWaveAt(catalog, identity, waveID)
		if err != nil {
			return err
		}
		if progress.State == "running" && record.State != "running" {
			return ErrMissionConflict
		}
		index := slices.IndexFunc(record.Children, func(child MissionExecutionChild) bool { return child.ID == childID && child.WaveID == waveID })
		if index < 0 {
			return ErrMissionConflict
		}
		child := &record.Children[index]
		if err := advanceMissionChild(child, progress); err != nil {
			return err
		}
		child.UpdatedAt, record.UpdatedAt = now, now
		record.Revision++
		catalog.Records[identity.MissionID] = record
		return nil
	})
}

func missionWaveAt(catalog *missionExecutions, identity MissionExecutionIdentity, waveID string) (MissionExecution, error) {
	record, exists := catalog.Records[identity.MissionID]
	if !exists || record.Mission.ProjectID != identity.ProjectID {
		return MissionExecution{}, ErrNotFound
	}
	if record.ID != identity.ExecutionID || record.Mission.PlanHash != identity.PlanHash || record.Wave == nil || record.Wave.ID != waveID {
		return MissionExecution{}, ErrMissionConflict
	}
	return record, nil
}

func advanceMissionChild(child *MissionExecutionChild, progress MissionChildProgress) error {
	if child.State != "reserved" && child.State != "running" {
		return ErrMissionConflict
	}
	if child.State == "running" && progress.State == "running" {
		return ErrMissionConflict
	}
	if !slices.Contains([]string{"running", "awaiting_review", "rejected", "failed", "interrupted"}, progress.State) {
		return fmt.Errorf("invalid mission child progress")
	}
	if progress.SessionID != "" {
		if child.SessionID != "" && (child.SessionID != progress.SessionID || child.WorkflowID != progress.WorkflowID || child.ProjectID != progress.ProjectID || child.ProjectRevision != progress.ProjectRevision) {
			return ErrMissionConflict
		}
		child.ProjectID, child.ProjectRevision = progress.ProjectID, progress.ProjectRevision
		child.SessionID, child.WorkflowID = progress.SessionID, progress.WorkflowID
		child.ProposalHash, child.ProposalRevision = progress.ProposalHash, progress.ProposalRevision
	}
	child.State, child.Reason = progress.State, progress.Reason
	return nil
}

func (s *Store) FinishMissionWave(ctx context.Context, identity MissionExecutionIdentity, waveID string, recovered bool, failure string, authorize func() error) (result MissionExecution, err error) {
	return s.finishMissionWave(ctx, identity, waveID, recovered, failure, nil, authorize)
}

// RecoverMissionWave atomically reconciles observed evidence against the exact
// receipt revision. Observation never grants another allocation or dispatch.
func (s *Store) RecoverMissionWave(ctx context.Context, identity MissionExecutionIdentity, waveID string, recovered bool, progress map[string]MissionChildProgress, authorize func() error) (MissionExecution, error) {
	if identity.Revision < 1 {
		return MissionExecution{}, ErrMissionConflict
	}
	return s.finishMissionWave(ctx, identity, waveID, recovered, "Recovered a stopped wave; uncertain allocations remain reserved and require inspection.", progress, authorize)
}

func (s *Store) finishMissionWave(ctx context.Context, identity MissionExecutionIdentity, waveID string, recovered bool, failure string, progress map[string]MissionChildProgress, authorize func() error) (result MissionExecution, err error) {
	err = s.mutateMissionExecutions(ctx, authorize, func(catalog *missionExecutions, now time.Time) error {
		record, err := missionWaveAt(catalog, identity, waveID)
		if err != nil {
			return err
		}
		if identity.Revision != 0 && record.Revision != identity.Revision || recovered && now.Before(record.Wave.Deadline()) {
			return ErrMissionConflict
		}
		if err := reconcileMissionChildren(&record, progress, now); err != nil {
			return err
		}
		elapsed := record.Wave.AllowanceMillis
		if !recovered {
			elapsed = min(elapsed, max(int64(1), now.Sub(record.Wave.StartedAt).Milliseconds()))
		}
		record.ElapsedMillis += elapsed
		for i := range record.Children {
			child := &record.Children[i]
			if child.WaveID == waveID && (child.State == "running" || child.State == "reserved") {
				child.State, child.UpdatedAt = "interrupted", now
				child.Reason = "Worker ownership ended before a complete receipt; the allocation remains reserved. Inspect retained worktree and proposal evidence."
			}
		}
		record.State = completedMissionWaveState(record)
		record.Failure = failure
		record.Wave, record.UpdatedAt = nil, now
		record.Revision++
		catalog.Records[identity.MissionID], result = record, record
		return nil
	})
	if err != nil {
		return MissionExecution{}, err
	}
	return result, nil
}

func reconcileMissionChildren(record *MissionExecution, progress map[string]MissionChildProgress, now time.Time) error {
	for id, evidence := range progress {
		index := slices.IndexFunc(record.Children, func(child MissionExecutionChild) bool { return child.ID == id && child.WaveID == record.Wave.ID })
		if index < 0 || evidence.State == "running" {
			return ErrMissionConflict
		}
		if err := advanceMissionChild(&record.Children[index], evidence); err != nil {
			return err
		}
		record.Children[index].UpdatedAt = now
	}
	return nil
}

func completedMissionWaveState(record MissionExecution) string {
	switch record.State {
	case "pausing":
		return "paused"
	case "canceling":
		return "canceled"
	}
	if slices.ContainsFunc(record.Children, func(child MissionExecutionChild) bool { return child.State == "awaiting_review" }) {
		return "awaiting_review"
	}
	return "blocked"
}
