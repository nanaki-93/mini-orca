package agentstate

import (
	"context"
	"crypto/rand"
	"fmt"
	"reflect"
	"slices"
	"time"
)

const MaxMissionRevalidations = 3

// A reservation retains both checkout identities before creating a candidate.
// Only a finished, checked candidate replaces the selected child proposal.
type MissionRevalidation struct {
	ID              string                  `json:"id"`
	Owner           string                  `json:"owner"`
	State           string                  `json:"state"`
	Previous        MissionExecutionChild   `json:"previous"`
	Source          MissionBase             `json:"source"`
	Evidence        MissionProposalEvidence `json:"evidence"`
	Before          MissionCheckpoint       `json:"before"`
	Candidate       MissionExecutionChild   `json:"candidate"`
	Failure         string                  `json:"failure,omitempty"`
	AllowanceMillis int64                   `json:"allowance_millis"`
	StartedAt       time.Time               `json:"started_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

func (entry MissionRevalidation) Deadline() time.Time {
	return entry.StartedAt.Add(time.Duration(entry.AllowanceMillis) * time.Millisecond)
}

func (record MissionExecution) PendingRevalidation() *MissionRevalidation {
	if len(record.Revalidations) == 0 {
		return nil
	}
	entry := record.Revalidations[len(record.Revalidations)-1]
	if entry.State != "pending" {
		return nil
	}
	return &entry
}

func missionCandidateDirectory(executionID, childID string, generation int) string {
	directory := executionID + "-" + childID
	if generation > 0 {
		directory += fmt.Sprintf("-r%d", generation)
	}
	return directory
}

func sameMissionCandidate(a, b MissionExecutionChild) bool {
	a.State, a.Reason, a.UpdatedAt = "", "", time.Time{}
	b.State, b.Reason, b.UpdatedAt = "", "", time.Time{}
	return a == b
}

func (entry MissionRevalidation) validate(record MissionExecution) error {
	if !validID(entry.ID) || !validID(entry.Owner) || !slices.Contains([]string{"pending", "awaiting_review", "failed", "interrupted"}, entry.State) {
		return ErrCorrupt
	}
	if entry.StartedAt.IsZero() || entry.UpdatedAt.Before(entry.StartedAt) || entry.AllowanceMillis < 1 || entry.AllowanceMillis > int64(entry.Previous.Limits.ElapsedSeconds)*1000 {
		return ErrCorrupt
	}
	if entry.Failure != "" && !validMemoryText(entry.Failure, 1024, true) {
		return ErrCorrupt
	}
	return entry.validateEvidence(record)
}

func (entry MissionRevalidation) validateEvidence(record MissionExecution) error {
	if entry.Previous.validate(record.ID) != nil || entry.Previous.SessionID == "" || entry.Previous.State != "awaiting_review" || entry.Candidate.validate(record.ID) != nil {
		return ErrCorrupt
	}
	if entry.Evidence.validate(record.ID, entry.Previous.ID) != nil || !validMissionBase(entry.Source) || entry.Source.Branch != entry.Previous.Branch || entry.Before.validate(record.Mission) != nil {
		return ErrCorrupt
	}
	return entry.validateCandidate()
}

func (entry MissionRevalidation) validateCandidate() error {
	a, b := entry.Previous, entry.Candidate
	if a.ID != b.ID || a.Limits != b.Limits || a.WaveID != b.WaveID || b.Generation <= a.Generation || b.WorkflowID != "" || b.BaseHead != entry.Before.Base.Head || b.BaseHead == a.BaseHead {
		return ErrCorrupt
	}
	if entry.State == "awaiting_review" && (b.State != "awaiting_review" || b.SessionID == "") {
		return ErrCorrupt
	}
	if entry.State != "awaiting_review" && b.State != "interrupted" {
		return ErrCorrupt
	}
	return nil
}

func (record MissionExecution) validateRevalidations() error {
	if len(record.Revalidations) > MaxMissionChildren*MaxMissionRevalidations {
		return ErrCorrupt
	}
	selected, generations, seen := map[string]MissionExecutionChild{}, map[string]int{}, map[string]bool{}
	for index, entry := range record.Revalidations {
		if entry.validate(record) != nil || seen[entry.ID] || !record.containsCheckpoint(entry.Before) {
			return ErrCorrupt
		}
		seen[entry.ID] = true
		id := entry.Previous.ID
		generations[id]++
		if generations[id] != entry.Candidate.Generation || generations[id] > MaxMissionRevalidations {
			return ErrCorrupt
		}
		previous, exists := selected[id]
		if exists && !sameMissionCandidate(previous, entry.Previous) || !exists && entry.Previous.Generation != 0 {
			return ErrCorrupt
		}
		selected[id] = entry.Previous
		if entry.State == "awaiting_review" {
			selected[id] = entry.Candidate
		}
		if entry.State == "pending" && !record.validRevalidationOwner(index) {
			return ErrCorrupt
		}
	}
	return record.validateSelectedCandidates(selected)
}

func (record MissionExecution) validRevalidationOwner(index int) bool {
	return index == len(record.Revalidations)-1 && record.Wave == nil && record.PendingIntegration() == nil && reflect.DeepEqual(record.Revalidations[index].Before, record.Current)
}

func (record MissionExecution) containsCheckpoint(checkpoint MissionCheckpoint) bool {
	if reflect.DeepEqual(initialMissionCheckpoint(record), checkpoint) {
		return true
	}
	return slices.ContainsFunc(record.Integrations, func(entry MissionIntegration) bool {
		return entry.State == "merged" && entry.After != nil && reflect.DeepEqual(*entry.After, checkpoint)
	})
}

func (record MissionExecution) validateSelectedCandidates(selected map[string]MissionExecutionChild) error {
	for id, candidate := range selected {
		if !slices.ContainsFunc(record.Children, func(child MissionExecutionChild) bool {
			return child.ID == id && sameMissionCandidate(child, candidate) && slices.Contains([]string{"awaiting_review", "integrated"}, child.State)
		}) {
			return ErrCorrupt
		}
	}
	for _, child := range record.Children {
		if child.Generation > 0 && selected[child.ID].SessionID != child.SessionID {
			return ErrCorrupt
		}
	}
	entry := record.PendingRevalidation()
	if record.State == "revalidating" && entry == nil || entry != nil && record.State != "revalidating" && record.State != "canceled" {
		return ErrCorrupt
	}
	if entry != nil && entry.AllowanceMillis+record.ElapsedMillis > int64(record.Mission.Plan.Limits.ElapsedSeconds)*1000 {
		return ErrCorrupt
	}
	return nil
}

func (record MissionExecution) admitRevalidation(childID string, source MissionBase, evidence MissionProposalEvidence) (MissionExecutionChild, int, error) {
	if record.Wave != nil || record.PendingIntegration() != nil || record.PendingRevalidation() != nil || record.State == "canceled" || record.State == "completed" {
		return MissionExecutionChild{}, 0, ErrMissionConflict
	}
	index := slices.IndexFunc(record.Children, func(child MissionExecutionChild) bool { return child.ID == childID })
	if index < 0 {
		return MissionExecutionChild{}, 0, ErrMissionConflict
	}
	child := record.Children[index]
	if child.State != "awaiting_review" || child.BaseHead == record.Current.Base.Head || source.Branch != child.Branch || !validMissionBase(source) || evidence.validate(record.ID, childID) != nil {
		return MissionExecutionChild{}, 0, ErrMissionConflict
	}
	generation := 1
	for _, entry := range record.Revalidations {
		if entry.Previous.ID == childID {
			generation++
		}
	}
	if generation > MaxMissionRevalidations {
		return MissionExecutionChild{}, 0, fmt.Errorf("mission child exhausted its three revalidation attempts")
	}
	return child, generation, nil
}

func (s *Store) BeginMissionRevalidation(ctx context.Context, identity MissionExecutionIdentity, owner, childID string, source MissionBase, evidence MissionProposalEvidence, authorize func() error) (result MissionExecution, err error) {
	err = s.mutateMissionExecutions(ctx, authorize, func(catalog *missionExecutions, now time.Time) error {
		record, err := missionExecutionAt(catalog, identity)
		if err != nil {
			return err
		}
		previous, generation, err := record.admitRevalidation(childID, source, evidence)
		if err != nil {
			return err
		}
		allowance := min(int64(previous.Limits.ElapsedSeconds)*1000, int64(record.Mission.Plan.Limits.ElapsedSeconds)*1000-record.ElapsedMillis)
		if allowance < 1 {
			return fmt.Errorf("mission elapsed allowance exhausted")
		}
		directory := missionCandidateDirectory(record.ID, childID, generation)
		candidate := MissionExecutionChild{ID: childID, State: "interrupted", Generation: generation, Limits: previous.Limits, WaveID: previous.WaveID, Directory: directory, Branch: "codex/" + directory, BaseHead: record.Current.Base.Head, UpdatedAt: now}
		entry := MissionRevalidation{ID: "revalidation-" + rand.Text(), Owner: owner, State: "pending", Previous: previous, Source: source, Evidence: evidence, Before: record.Current, Candidate: candidate, AllowanceMillis: allowance, StartedAt: now, UpdatedAt: now}
		record.Revalidations = append(record.Revalidations, entry)
		record.State, record.Failure, record.UpdatedAt = "revalidating", "", now
		record.Revision++
		catalog.Records[identity.MissionID], result = record, record
		return nil
	})
	return result, err
}

func missionRevalidationAt(catalog *missionExecutions, identity MissionExecutionIdentity, id string) (MissionExecution, error) {
	record, exists := catalog.Records[identity.MissionID]
	if !exists || record.Mission.ProjectID != identity.ProjectID {
		return MissionExecution{}, ErrNotFound
	}
	if record.ID != identity.ExecutionID || record.Mission.PlanHash != identity.PlanHash || identity.Revision != 0 && record.Revision != identity.Revision {
		return MissionExecution{}, ErrMissionConflict
	}
	entry := record.PendingRevalidation()
	if entry == nil || entry.ID != id {
		return MissionExecution{}, ErrMissionConflict
	}
	return record, nil
}

// Progress publishes identity before checks. It conveys no check or review authority.
func (s *Store) RecordMissionRevalidationCandidate(ctx context.Context, identity MissionExecutionIdentity, id string, progress MissionChildProgress, authorize func() error) (err error) {
	return s.mutateMissionExecutions(ctx, authorize, func(catalog *missionExecutions, now time.Time) error {
		record, err := missionRevalidationAt(catalog, identity, id)
		if err != nil {
			return err
		}
		entry := &record.Revalidations[len(record.Revalidations)-1]
		if record.State != "revalidating" || entry.Candidate.SessionID != "" || progress.WorkflowID != "" || progress.SessionID == "" {
			return ErrMissionConflict
		}
		child := &entry.Candidate
		child.ProjectID, child.ProjectRevision, child.SessionID = progress.ProjectID, progress.ProjectRevision, progress.SessionID
		child.ProposalHash, child.ProposalRevision, child.UpdatedAt = progress.ProposalHash, progress.ProposalRevision, now
		entry.UpdatedAt, record.UpdatedAt = now, now
		record.Revision++
		catalog.Records[identity.MissionID] = record
		return nil
	})
}

func (s *Store) FinishMissionRevalidation(ctx context.Context, identity MissionExecutionIdentity, id, state, failure string, recovered bool, authorize func() error) (result MissionExecution, err error) {
	err = s.mutateMissionExecutions(ctx, authorize, func(catalog *missionExecutions, now time.Time) error {
		record, err := missionRevalidationAt(catalog, identity, id)
		if err != nil {
			return err
		}
		if !slices.Contains([]string{"awaiting_review", "failed", "interrupted"}, state) || state == "awaiting_review" && (record.State != "revalidating" || recovered) {
			return ErrMissionConflict
		}
		entry := &record.Revalidations[len(record.Revalidations)-1]
		entry.State, entry.Failure, entry.UpdatedAt = state, failure, now
		if state == "awaiting_review" {
			entry.Candidate.State, entry.Candidate.UpdatedAt = state, now
			index := slices.IndexFunc(record.Children, func(child MissionExecutionChild) bool { return child.ID == entry.Candidate.ID })
			record.Children[index] = entry.Candidate
		}
		elapsed := max(int64(0), now.Sub(entry.StartedAt).Milliseconds())
		if recovered {
			elapsed = entry.AllowanceMillis
		}
		record.ElapsedMillis += min(entry.AllowanceMillis, elapsed)
		if record.State != "canceled" {
			record.State = integratedMissionState(record)
		}
		record.Failure, record.UpdatedAt = failure, now
		record.Revision++
		catalog.Records[identity.MissionID], result = record, record
		return nil
	})
	return result, err
}

func upgradeMissionRevalidations(catalog *missionExecutions) error {
	if catalog.Version != 3 {
		return nil
	}
	for _, record := range catalog.Records {
		if len(record.Revalidations) != 0 || record.State == "revalidating" || slices.ContainsFunc(record.Children, func(child MissionExecutionChild) bool { return child.Generation != 0 }) {
			return ErrCorrupt
		}
	}
	catalog.Version = 4
	return nil
}
