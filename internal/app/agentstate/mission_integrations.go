package agentstate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"time"
)

const MaxMissionIntegrations = MaxMissionChildren * 8

func upgradeMissionIntegrationBindings(catalog *missionExecutions) error {
	if catalog.Version != 2 {
		return nil
	}
	for id, record := range catalog.Records {
		if !reflect.DeepEqual(record.Current, MissionCheckpoint{}) || len(record.Integrations) != 0 || slices.Contains([]string{"integrating", "ready", "completed"}, record.State) {
			return ErrCorrupt
		}
		record.Current = initialMissionCheckpoint(record)
		catalog.Records[id] = record
	}
	catalog.Version = 3
	return nil
}

// MissionCheckpoint records only an observed, explicitly integrated base. The
// approved mission remains immutable, including its original source bindings.
type MissionCheckpoint struct {
	ProjectRevision string            `json:"project_revision"`
	Base            MissionBase       `json:"base"`
	Sources         []MissionSource   `json:"sources"`
	Instructions    map[string]string `json:"instructions"`
}

func initialMissionCheckpoint(record MissionExecution) MissionCheckpoint {
	return MissionCheckpoint{ProjectRevision: record.Mission.ProjectRevision, Base: record.Base, Sources: slices.Clone(record.Mission.Plan.Sources), Instructions: maps.Clone(record.Mission.Instructions)}
}

func (checkpoint MissionCheckpoint) validate(mission MissionRecord) error {
	if !validID(checkpoint.ProjectRevision) || !validMissionBase(checkpoint.Base) || len(checkpoint.Sources) != len(mission.Plan.Sources) || len(checkpoint.Instructions) != len(checkpoint.Sources) {
		return ErrCorrupt
	}
	if _, err := missionSources(checkpoint.Sources); err != nil {
		return ErrCorrupt
	}
	for index, source := range checkpoint.Sources {
		if source.Path != mission.Plan.Sources[index].Path || (MemorySource{Path: source.Path, Hash: checkpoint.Instructions[source.Path]}).Validate() != nil {
			return ErrCorrupt
		}
	}
	return nil
}

// BoundMission is a transient source binding for host admission. It is not a
// revised plan and must never replace the approved catalog or its plan hash.
func (record MissionExecution) BoundMission() MissionRecord {
	bound := record.Mission
	bound.ProjectRevision = record.Current.ProjectRevision
	bound.Plan.Sources = slices.Clone(record.Current.Sources)
	bound.Instructions = maps.Clone(record.Current.Instructions)
	return bound
}

type MissionProposalEvidence struct {
	Key     ArtifactKey `json:"key"`
	Version int64       `json:"version"`
	Hash    string      `json:"hash"`
}

func (evidence MissionProposalEvidence) validate(executionID, childID string) error {
	expected := ArtifactKey{Session: Key{App: "mini-orca-missions", User: "local", ID: executionID}, Name: childID + "-proposal.json"}
	if evidence.Key != expected || evidence.Version < 1 || evidence.Version > 1<<53 || len(evidence.Hash) != 64 {
		return ErrCorrupt
	}
	if _, err := hex.DecodeString(evidence.Hash); err != nil {
		return ErrCorrupt
	}
	return nil
}

type MissionIntegration struct {
	ID               string                  `json:"id"`
	ChildID          string                  `json:"child_id"`
	Owner            string                  `json:"owner"`
	State            string                  `json:"state"`
	Source           MissionBase             `json:"source"`
	SessionID        string                  `json:"session_id"`
	WorkflowID       string                  `json:"workflow_id"`
	ProposalHash     string                  `json:"proposal_hash"`
	ProposalRevision int64                   `json:"proposal_revision"`
	Evidence         MissionProposalEvidence `json:"evidence"`
	Before           MissionCheckpoint       `json:"before"`
	After            *MissionCheckpoint      `json:"after,omitempty"`
	Failure          string                  `json:"failure,omitempty"`
	Cleanup          string                  `json:"cleanup"`
	StartedAt        time.Time               `json:"started_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
}

func (integration MissionIntegration) Deadline() time.Time {
	return integration.StartedAt.Add(3 * time.Minute)
}

func (integration MissionIntegration) unresolved() bool {
	return integration.State == "pending" || integration.State == "conflicts" || integration.State == "unavailable"
}

func (record MissionExecution) PendingIntegration() *MissionIntegration {
	if len(record.Integrations) == 0 {
		return nil
	}
	last := record.Integrations[len(record.Integrations)-1]
	if !last.unresolved() {
		return nil
	}
	return &last
}

func (integration MissionIntegration) validate(record MissionExecution) error {
	child, exists := record.integrationCandidate(integration)
	if !exists || integration.Source.Branch != child.Branch || integration.Source.Head == integration.Before.Base.Head || child.BaseHead != integration.Before.Base.Head {
		return ErrCorrupt
	}
	for _, value := range []string{integration.ID, integration.ChildID, integration.Owner, integration.SessionID, integration.ProposalHash} {
		if !validID(value) {
			return ErrCorrupt
		}
	}
	if !validMissionBase(integration.Source) || integration.ProposalRevision < 0 || integration.StartedAt.IsZero() || integration.UpdatedAt.Before(integration.StartedAt) {
		return ErrCorrupt
	}
	if integration.Evidence.validate(record.ID, integration.ChildID) != nil || integration.Before.validate(record.Mission) != nil {
		return ErrCorrupt
	}
	return integration.validateOutcome(record)
}

func (record MissionExecution) integrationCandidate(entry MissionIntegration) (MissionExecutionChild, bool) {
	for _, child := range record.Children {
		if child.ID == entry.ChildID && entry.matchesChildProposal(child) {
			return child, true
		}
	}
	for _, attempt := range record.Revalidations {
		if attempt.Previous.ID == entry.ChildID && entry.matchesChildProposal(attempt.Previous) {
			return attempt.Previous, true
		}
	}
	return MissionExecutionChild{}, false
}

func (integration MissionIntegration) matchesChildProposal(child MissionExecutionChild) bool {
	return integration.SessionID == child.SessionID && integration.WorkflowID == child.WorkflowID && integration.ProposalHash == child.ProposalHash && integration.ProposalRevision == child.ProposalRevision
}

// MissionWorktree finds reserved ownership even before a proposal exists. An
// empty or missing checkout cannot discard the durable reservation.
func (s *Store) MissionWorktree(ctx context.Context, projectID, directory, branch string) (MissionExecution, error) {
	if err := context.Cause(ctx); err != nil {
		return MissionExecution{}, err
	}
	catalog, err := s.loadMissionExecutions()
	if err != nil {
		return MissionExecution{}, err
	}
	for _, record := range catalog.Records {
		if record.Mission.ProjectID == projectID && record.ownsMissionWorktree(directory, branch) {
			return record, nil
		}
	}
	return MissionExecution{}, ErrNotFound
}

func (record MissionExecution) ownsMissionWorktree(directory, branch string) bool {
	matches := func(child MissionExecutionChild) bool {
		return directory != "" && child.Directory == directory || branch != "" && child.Branch == branch
	}
	if slices.ContainsFunc(record.Children, matches) {
		return true
	}
	return slices.ContainsFunc(record.Revalidations, func(entry MissionRevalidation) bool {
		return matches(entry.Previous) || matches(entry.Candidate)
	})
}

func (s *Store) RecordMissionCleanup(ctx context.Context, identity MissionExecutionIdentity, integrationID, state, failure string, authorize func() error) (result MissionExecution, err error) {
	err = s.mutateMissionExecutions(ctx, authorize, func(catalog *missionExecutions, now time.Time) error {
		record, err := missionExecutionAt(catalog, identity)
		if err != nil {
			return err
		}
		index := slices.IndexFunc(record.Integrations, func(entry MissionIntegration) bool { return entry.ID == integrationID })
		if index < 0 || record.Integrations[index].State != "merged" || !slices.Contains([]string{"worktree_removed", "removed", "failed"}, state) {
			return ErrMissionConflict
		}
		entry := &record.Integrations[index]
		if entry.Cleanup == "removed" || entry.Cleanup == "worktree_removed" && state == "failed" {
			return ErrMissionConflict
		}
		entry.Cleanup, entry.Failure, entry.UpdatedAt = state, failure, now
		record.UpdatedAt = now
		record.Revision++
		catalog.Records[identity.MissionID], result = record, record
		return nil
	})
	if err != nil {
		return MissionExecution{}, err
	}
	return result, nil
}

func (integration MissionIntegration) validateOutcome(record MissionExecution) error {
	if !slices.Contains([]string{"pending", "merged", "failed", "conflicts", "unavailable"}, integration.State) || !slices.Contains([]string{"retained", "pending", "worktree_removed", "removed", "failed"}, integration.Cleanup) {
		return ErrCorrupt
	}
	if integration.Failure != "" && !validMemoryText(integration.Failure, 1024, true) {
		return ErrCorrupt
	}
	if integration.State == "merged" {
		if integration.After == nil || integration.After.validate(record.Mission) != nil || integration.After.Base.Branch != integration.Before.Base.Branch || integration.After.Base.Head != integration.Source.Head {
			return ErrCorrupt
		}
	} else if integration.After != nil || integration.Cleanup != "retained" {
		return ErrCorrupt
	}
	return nil
}

func (record MissionExecution) validateIntegrations() error {
	if record.Current.validate(record.Mission) != nil || len(record.Integrations) > MaxMissionIntegrations {
		return ErrCorrupt
	}
	checkpoint := initialMissionCheckpoint(record)
	integrated := map[string]bool{}
	seen := map[string]bool{}
	for index, entry := range record.Integrations {
		if entry.validate(record) != nil || seen[entry.ID] || !reflect.DeepEqual(checkpoint, entry.Before) || integrated[entry.ChildID] {
			return ErrCorrupt
		}
		seen[entry.ID] = true
		if entry.unresolved() && (index != len(record.Integrations)-1 || record.Wave != nil) {
			return ErrCorrupt
		}
		if entry.State == "merged" {
			if err := record.validateIntegratedChild(entry, integrated); err != nil {
				return err
			}
			integrated[entry.ChildID] = true
			checkpoint = *entry.After
		}
	}
	return record.validateIntegrationCheckpoint(checkpoint, integrated)
}

func (record MissionExecution) validateIntegratedChild(entry MissionIntegration, integrated map[string]bool) error {
	position := len(integrated)
	if position >= len(record.Mission.Plan.IntegrationOrder) || record.Mission.Plan.IntegrationOrder[position] != entry.ChildID {
		return ErrCorrupt
	}
	index := slices.IndexFunc(record.Mission.Plan.Children, func(child MissionChild) bool { return child.ID == entry.ChildID })
	if index < 0 {
		return ErrCorrupt
	}
	child := record.Mission.Plan.Children[index]
	for index, source := range entry.After.Sources {
		if !slices.Contains(child.WritePaths, source.Path) && source != entry.Before.Sources[index] {
			return ErrCorrupt
		}
	}
	return nil
}

func (record MissionExecution) validateIntegrationCheckpoint(checkpoint MissionCheckpoint, integrated map[string]bool) error {
	if !reflect.DeepEqual(record.Current, checkpoint) {
		return ErrCorrupt
	}
	for _, child := range record.Children {
		if (child.State == "integrated") != integrated[child.ID] {
			return ErrCorrupt
		}
	}
	if record.State == "completed" && len(integrated) != len(record.Children) {
		return ErrCorrupt
	}
	if record.State == "integrating" && record.PendingIntegration() == nil {
		return ErrCorrupt
	}
	return nil
}

type MissionIntegrationInput struct {
	ChildID          string
	Source           MissionBase
	SessionID        string
	WorkflowID       string
	ProposalHash     string
	ProposalRevision int64
	Evidence         MissionProposalEvidence
}

func (record MissionExecution) admitIntegration(input MissionIntegrationInput) error {
	if record.Wave != nil || record.PendingIntegration() != nil || record.PendingRevalidation() != nil || record.State == "canceled" || record.State == "completed" || len(record.Integrations) >= MaxMissionIntegrations {
		return ErrMissionConflict
	}
	for _, id := range record.Mission.Plan.IntegrationOrder {
		index := slices.IndexFunc(record.Children, func(child MissionExecutionChild) bool { return child.ID == id })
		child := record.Children[index]
		if child.State == "integrated" {
			continue
		}
		return validateIntegrationInput(record, child, input)
	}
	return ErrMissionConflict
}

func validateIntegrationInput(record MissionExecution, child MissionExecutionChild, input MissionIntegrationInput) error {
	if input.ChildID != child.ID || child.State != "awaiting_review" || child.BaseHead != record.Current.Base.Head {
		return fmt.Errorf("%w: integrate the next reviewed child against the recorded current base; older candidates require revalidation", ErrMissionConflict)
	}
	if input.SessionID != child.SessionID || input.WorkflowID != child.WorkflowID || input.ProposalHash != child.ProposalHash || input.ProposalRevision != child.ProposalRevision || input.Source.Branch != child.Branch {
		return ErrMissionConflict
	}
	if !validMissionBase(input.Source) || input.Source.Head == child.BaseHead || input.Evidence.validate(record.ID, child.ID) != nil {
		return fmt.Errorf("integration requires an exact committed proposal and retained private evidence")
	}
	return nil
}

func (s *Store) BeginMissionIntegration(ctx context.Context, identity MissionExecutionIdentity, owner string, input MissionIntegrationInput, authorize func() error) (result MissionExecution, err error) {
	err = s.mutateMissionExecutions(ctx, authorize, func(catalog *missionExecutions, now time.Time) error {
		record, err := missionExecutionAt(catalog, identity)
		if err != nil {
			return err
		}
		if err := record.admitIntegration(input); err != nil {
			return err
		}
		entry := MissionIntegration{ID: "integration-" + rand.Text(), ChildID: input.ChildID, Owner: owner, State: "pending", Source: input.Source, SessionID: input.SessionID, WorkflowID: input.WorkflowID, ProposalHash: input.ProposalHash, ProposalRevision: input.ProposalRevision, Evidence: input.Evidence, Before: record.Current, Cleanup: "retained", StartedAt: now, UpdatedAt: now}
		record.Integrations = append(record.Integrations, entry)
		record.State, record.Failure, record.UpdatedAt = "integrating", "", now
		record.Revision++
		catalog.Records[identity.MissionID], result = record, record
		return nil
	})
	if err != nil {
		return MissionExecution{}, err
	}
	return result, nil
}

type MissionIntegrationOutcome struct {
	State   string
	After   *MissionCheckpoint
	Failure string
}

func (s *Store) FinishMissionIntegration(ctx context.Context, identity MissionExecutionIdentity, integrationID string, outcome MissionIntegrationOutcome, authorize func() error) (result MissionExecution, err error) {
	err = s.mutateMissionExecutions(ctx, authorize, func(catalog *missionExecutions, now time.Time) error {
		record, err := missionIntegrationAt(catalog, identity, integrationID)
		if err != nil {
			return err
		}
		if err := completeMissionIntegration(&record, outcome, now); err != nil {
			return err
		}
		catalog.Records[identity.MissionID], result = record, record
		return nil
	})
	if err != nil {
		return MissionExecution{}, err
	}
	return result, nil
}

func missionIntegrationAt(catalog *missionExecutions, identity MissionExecutionIdentity, id string) (MissionExecution, error) {
	record, exists := catalog.Records[identity.MissionID]
	if !exists || record.Mission.ProjectID != identity.ProjectID {
		return MissionExecution{}, ErrNotFound
	}
	if record.ID != identity.ExecutionID || record.Mission.PlanHash != identity.PlanHash || identity.Revision != 0 && record.Revision != identity.Revision {
		return MissionExecution{}, ErrMissionConflict
	}
	entry := record.PendingIntegration()
	if entry == nil || entry.ID != id {
		return MissionExecution{}, ErrMissionConflict
	}
	return record, nil
}

func completeMissionIntegration(record *MissionExecution, outcome MissionIntegrationOutcome, now time.Time) error {
	if !slices.Contains([]string{"merged", "failed", "conflicts", "unavailable"}, outcome.State) {
		return fmt.Errorf("invalid observed integration outcome")
	}
	entry := &record.Integrations[len(record.Integrations)-1]
	entry.State, entry.After, entry.Failure, entry.UpdatedAt = outcome.State, outcome.After, outcome.Failure, now
	if outcome.State == "merged" {
		if outcome.After == nil {
			return ErrMissionConflict
		}
		record.Current, entry.Cleanup = *outcome.After, "pending"
		index := slices.IndexFunc(record.Children, func(child MissionExecutionChild) bool { return child.ID == entry.ChildID })
		record.Children[index].State, record.Children[index].UpdatedAt = "integrated", now
	}
	if record.State != "canceled" {
		record.State = integratedMissionState(*record)
	}
	record.Failure, record.UpdatedAt = outcome.Failure, now
	record.Revision++
	return nil
}

func integratedMissionState(record MissionExecution) string {
	if record.PendingIntegration() != nil {
		return "blocked"
	}
	if len(record.ReadyChildren()) > 0 {
		return "ready"
	}
	if slices.ContainsFunc(record.Children, func(child MissionExecutionChild) bool { return child.State == "awaiting_review" }) {
		return "awaiting_review"
	}
	if !slices.ContainsFunc(record.Children, func(child MissionExecutionChild) bool { return child.State != "integrated" }) {
		return "completed"
	}
	return "blocked"
}
