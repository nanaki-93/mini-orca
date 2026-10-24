package agentstate

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrDelegationConflict = errors.New("remote delegation changed or requires recovery")

type RemoteArtifact struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Text     string `json:"text"`
	Complete bool   `json:"complete"`
}

type RemoteUsage struct {
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	Source       string `json:"source"` // unknown or remote_reported; never independently measured.
}

// RemoteProgress contains advisory text only. Remote metadata cannot describe
// local permissions, tools, source writes or executable artifact paths.
type RemoteProgress struct {
	TaskID    string           `json:"task_id,omitempty"`
	ContextID string           `json:"context_id,omitempty"`
	State     string           `json:"state"`
	Text      string           `json:"text,omitempty"`
	Artifacts []RemoteArtifact `json:"artifacts"`
	Usage     RemoteUsage      `json:"usage"`
}

type Delegation struct {
	ID              string         `json:"id"`
	Version         int64          `json:"version"`
	ProjectID       string         `json:"project_id"`
	ProjectRevision string         `json:"project_revision"`
	RemoteID        string         `json:"remote_id"`
	EndpointHash    string         `json:"endpoint_hash"`
	CardFingerprint string         `json:"card_fingerprint"`
	Skill           string         `json:"skill"`
	Kind            string         `json:"kind"`
	Question        string         `json:"question"`
	Policy          string         `json:"policy"`
	Sources         []MemorySource `json:"sources"`
	InputHash       string         `json:"input_hash"`
	MessageID       string         `json:"message_id"`
	Operation       string         `json:"operation"`
	AttemptID       string         `json:"attempt_id"`
	State           string         `json:"state"`
	Progress        RemoteProgress `json:"progress"`
	Observation     string         `json:"observation,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	LeaseUntil      time.Time      `json:"lease_until"`
}

func RemoteTerminal(state string) bool {
	return slices.Contains([]string{"completed", "failed", "rejected", "canceled", "not_sent", "reconciled"}, state)
}

func (p RemoteProgress) Validate() error {
	if !slices.Contains([]string{"submitted", "working", "input_required", "auth_required", "completed", "failed", "rejected", "canceled", "unknown", "not_sent"}, p.State) {
		return ErrCorrupt
	}
	if p.TaskID != "" && !validID(p.TaskID) || p.ContextID != "" && !validID(p.ContextID) {
		return ErrCorrupt
	}
	if p.TaskID != "" && p.ContextID == "" {
		return ErrCorrupt
	}
	if !remoteText(p.Text, 64<<10) || len(p.Artifacts) > 4 {
		return ErrCorrupt
	}
	if err := p.validateArtifacts(); err != nil {
		return err
	}
	return p.Usage.validate()
}

func (p RemoteProgress) validateArtifacts() error {
	total := len(p.Text)
	seen := map[string]bool{}
	for _, artifact := range p.Artifacts {
		if !validID(artifact.ID) || seen[artifact.ID] || !remoteText(artifact.Name, 256) || !remoteText(artifact.Text, 64<<10) {
			return ErrCorrupt
		}
		seen[artifact.ID] = true
		if p.State == "completed" && !artifact.Complete {
			return ErrCorrupt
		}
		total += len(artifact.Text)
	}
	if total > 64<<10 {
		return ErrCorrupt
	}
	return nil
}

func remoteText(value string, limit int) bool {
	return len(value) <= limit && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func (u RemoteUsage) validate() error {
	if u.Source != "unknown" && u.Source != "remote_reported" {
		return ErrCorrupt
	}
	if u.InputTokens < 0 || u.OutputTokens < 0 || u.InputTokens > 1<<40 || u.OutputTokens > 1<<40 {
		return ErrCorrupt
	}
	if u.Source == "unknown" && (u.InputTokens != 0 || u.OutputTokens != 0) {
		return ErrCorrupt
	}
	return nil
}

func (d Delegation) validate() error {
	for _, id := range []string{d.ID, d.ProjectID, d.ProjectRevision, d.RemoteID, d.MessageID, d.AttemptID, d.Skill} {
		if !validID(id) {
			return ErrCorrupt
		}
	}
	for _, hash := range []string{d.EndpointHash, d.CardFingerprint, d.InputHash} {
		if !validHash(hash) {
			return ErrCorrupt
		}
	}
	if d.Version < 1 || d.Version > 1<<53 || d.CreatedAt.IsZero() || d.UpdatedAt.Before(d.CreatedAt) {
		return ErrCorrupt
	}
	if !d.validRequest() {
		return ErrCorrupt
	}
	if err := validateDelegationSources(d.Sources); err != nil {
		return err
	}
	if err := d.Progress.Validate(); err != nil {
		return err
	}
	return validateDelegationPhase(d)
}

func (d Delegation) validRequest() bool {
	return (d.Kind == "analysis" || d.Kind == "review") && validMemoryText(d.Question, 2048, true) && remoteText(d.Policy, 256) && d.Policy != ""
}

func validateDelegationSources(sources []MemorySource) error {
	if len(sources) > 8 {
		return ErrCorrupt
	}
	seen := map[string]bool{}
	for _, source := range sources {
		if source.Validate() != nil || source.StartLine != 0 || source.EndLine != 0 || seen[source.Path] {
			return ErrCorrupt
		}
		seen[source.Path] = true
	}
	return nil
}

func validateDelegationPhase(d Delegation) error {
	if !slices.Contains([]string{"start", "input", "refresh", "reconnect", "cancel", "reconcile"}, d.Operation) {
		return ErrCorrupt
	}
	if d.State == "pending" {
		if d.LeaseUntil.IsZero() {
			return ErrCorrupt
		}
	} else if d.State != "reconciled" && d.State != d.Progress.State {
		return ErrCorrupt
	}
	if d.State == "reconciled" {
		if !validMemoryText(d.Observation, 2048, true) {
			return ErrCorrupt
		}
	} else if d.Observation != "" {
		return ErrCorrupt
	}
	return nil
}

func validateDelegations(db *database) error {
	if len(db.Delegations) > 128 {
		return fmt.Errorf("remote delegation history limit reached")
	}
	for id, record := range db.Delegations {
		if id != record.ID || record.validate() != nil {
			return ErrCorrupt
		}
	}
	return nil
}

func cloneDelegation(d Delegation) Delegation {
	d.Sources = slices.Clone(d.Sources)
	d.Progress.Artifacts = slices.Clone(d.Progress.Artifacts)
	return d
}

func (s *Store) Delegations(ctx context.Context, projectID string) ([]Delegation, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	db, err := s.load()
	if err != nil {
		return nil, err
	}
	result := []Delegation{}
	for _, record := range db.Delegations {
		if record.ProjectID == projectID {
			result = append(result, cloneDelegation(record))
		}
	}
	slices.SortFunc(result, func(a, b Delegation) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return result, nil
}

func validRemoteLease(deadline, now time.Time) bool {
	return deadline.After(now) && !deadline.After(now.Add(90*time.Second))
}

func (s *Store) CreateDelegation(ctx context.Context, draft Delegation, deadline time.Time, authorize func() error) (*Delegation, error) {
	now := time.Now().UTC()
	if !validRemoteLease(deadline, now) {
		return nil, ErrDelegationConflict
	}
	draft.ID, draft.MessageID, draft.AttemptID = rand.Text(), rand.Text(), rand.Text()
	draft.Version, draft.Operation, draft.State = 1, "start", "pending"
	draft.CreatedAt, draft.UpdatedAt, draft.LeaseUntil = now, now, deadline
	draft.Progress = RemoteProgress{State: "unknown", Artifacts: []RemoteArtifact{}, Usage: RemoteUsage{Source: "unknown"}}
	draft = cloneDelegation(draft)
	err := s.mutate(ctx, func(db *database) error {
		if err := actionGuard(ctx, authorize); err != nil {
			return err
		}
		for _, record := range db.Delegations {
			if record.EndpointHash == draft.EndpointHash && !RemoteTerminal(record.State) {
				return ErrDelegationConflict
			}
		}
		if db.Delegations == nil {
			db.Delegations = map[string]Delegation{}
		}
		db.Delegations[draft.ID] = draft
		return nil
	})
	return &draft, err
}

func expectedDelegation(db *database, id string, version int64) (Delegation, error) {
	d, ok := db.Delegations[id]
	if !ok {
		return d, ErrNotFound
	}
	if d.Version != version {
		return d, ErrDelegationConflict
	}
	return d, nil
}

func canClaimDelegation(d Delegation, operation string, now time.Time) bool {
	if RemoteTerminal(d.State) || d.State == "pending" && now.Before(d.LeaseUntil) || d.Progress.TaskID == "" {
		return false
	}
	switch operation {
	case "input":
		return d.State == "input_required"
	case "refresh", "reconnect", "cancel":
		return true
	default:
		return false
	}
}

func (s *Store) ClaimDelegation(ctx context.Context, id string, version int64, operation, inputHash string, deadline time.Time, authorize func() error) (*Delegation, error) {
	var result Delegation
	err := s.mutate(ctx, func(db *database) error {
		if err := actionGuard(ctx, authorize); err != nil {
			return err
		}
		d, err := expectedDelegation(db, id, version)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if !validRemoteLease(deadline, now) || !canClaimDelegation(d, operation, now) {
			return ErrDelegationConflict
		}
		d.AttemptID, d.Operation, d.State = rand.Text(), operation, "pending"
		d.LeaseUntil, d.UpdatedAt = deadline, now
		if operation == "input" {
			d.MessageID, d.InputHash = rand.Text(), inputHash
		}
		d.Version++
		db.Delegations[id], result = d, cloneDelegation(d)
		return nil
	})
	return &result, err
}

// SaveDelegationProgress is independent of current source authority so remote
// task identity and known outcomes survive cancellation, reindex and revocation.
func (s *Store) SaveDelegationProgress(ctx context.Context, id, attempt string, progress RemoteProgress, finished bool) (*Delegation, error) {
	var result Delegation
	progress.Artifacts = slices.Clone(progress.Artifacts)
	err := s.mutate(ctx, func(db *database) error {
		d, ok := db.Delegations[id]
		if !ok || d.AttemptID != attempt || d.State != "pending" {
			return ErrDelegationConflict
		}
		if err := sameRemoteIdentity(d.Progress, progress); err != nil {
			return err
		}
		d.Progress = progress
		d.Version++
		d.UpdatedAt = time.Now().UTC()
		if finished {
			d.State = progress.State
			d.LeaseUntil = time.Time{}
		}
		db.Delegations[id], result = d, cloneDelegation(d)
		return nil
	})
	return &result, err
}

func sameRemoteIdentity(previous, current RemoteProgress) error {
	if previous.TaskID != "" && previous.TaskID != current.TaskID || previous.ContextID != "" && previous.ContextID != current.ContextID {
		return ErrDelegationConflict
	}
	return current.Validate()
}

func (s *Store) ReconcileDelegation(ctx context.Context, id string, version int64, observation string, authorize func() error) (*Delegation, error) {
	var result Delegation
	err := s.mutate(ctx, func(db *database) error {
		if err := actionGuard(ctx, authorize); err != nil {
			return err
		}
		d, err := expectedDelegation(db, id, version)
		if err != nil {
			return err
		}
		if d.State != "unknown" && d.State != "auth_required" && !(d.State == "pending" && time.Now().After(d.LeaseUntil)) {
			return ErrDelegationConflict
		}
		d.State, d.Operation, d.Observation = "reconciled", "reconcile", observation
		d.UpdatedAt, d.LeaseUntil = time.Now().UTC(), time.Time{}
		d.Version++
		db.Delegations[id], result = d, cloneDelegation(d)
		return nil
	})
	return &result, err
}
