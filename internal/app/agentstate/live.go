package agentstate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

var ErrLiveConflict = errors.New("live session changed; refresh its controls")

// LiveRecord is a local, source-free receipt. It does not contain provider
// resumption tokens, grant credentials, prompts, transcripts or audio.
type LiveRecord struct {
	ID              string              `json:"id"`
	Version         int64               `json:"version"`
	ControlRevision int64               `json:"control_revision"`
	ProjectID       string              `json:"project_id"`
	ProjectRevision string              `json:"project_revision"`
	Policy          string              `json:"policy"`
	Configuration   string              `json:"configuration"`
	SettingsVersion int64               `json:"settings_version"`
	Model           string              `json:"model"`
	ProviderOrigin  string              `json:"provider_origin"`
	State           string              `json:"state"`
	Connections     int                 `json:"connections"`
	Turns           int                 `json:"turns"`
	CompletedTurns  int                 `json:"completed_turns"`
	SentBytes       int64               `json:"sent_bytes"`
	ReceivedBytes   int64               `json:"received_bytes"`
	ReportedInput   int64               `json:"reported_input_tokens"`
	ReportedOutput  int64               `json:"reported_output_tokens"`
	Reported        bool                `json:"usage_reported"`
	Usage           BudgetUsage         `json:"usage"`
	RunIDs          []string            `json:"run_ids"`
	Artifacts       []ArtifactReference `json:"artifacts"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	ExpiresAt       time.Time           `json:"expires_at"`
}

type liveCatalog struct {
	Version   int                   `json:"version"`
	Workspace string                `json:"workspace"`
	Records   map[string]LiveRecord `json:"records"`
}

func (r LiveRecord) validate() error {
	for _, check := range []func() error{r.validateBinding, r.validateProgress, r.validateReferences} {
		if err := check(); err != nil {
			return err
		}
	}
	if r.CreatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) || !r.ExpiresAt.After(r.CreatedAt) || r.ExpiresAt.Sub(r.CreatedAt) > 5*time.Minute {
		return ErrCorrupt
	}
	return nil
}

func (r LiveRecord) validateBinding() error {
	if !validID(r.ID) || (Binding{Project: r.ProjectID, Revision: r.ProjectRevision, Policy: r.Policy, Configuration: r.Configuration}).Validate() != nil || !validID(r.Model) {
		return ErrCorrupt
	}
	if r.Version < 1 || r.Version > 1<<53 || r.ControlRevision < 1 || r.ControlRevision > r.Version || r.SettingsVersion < 0 || !validMemoryText(r.ProviderOrigin, 512, false) {
		return ErrCorrupt
	}
	if !slices.Contains([]string{"connecting", "connected", "disconnected", "go_away", "closed", "interrupted", "limit_reached", "failed"}, r.State) {
		return ErrCorrupt
	}
	return nil
}

func (r LiveRecord) validateProgress() error {
	if r.Connections < 1 || r.Connections > 2 || r.Turns < 0 || r.Turns > 16 || r.CompletedTurns < 0 || r.CompletedTurns > 512 {
		return ErrCorrupt
	}
	for _, n := range []int64{r.SentBytes, r.ReceivedBytes, r.ReportedInput, r.ReportedOutput} {
		if n < 0 || n > 1<<40 {
			return ErrCorrupt
		}
	}
	if !r.Reported && (r.ReportedInput != 0 || r.ReportedOutput != 0) || !validUsage(r.Usage) {
		return ErrCorrupt
	}
	return nil
}

func (r LiveRecord) validateReferences() error {
	if len(r.RunIDs) > 2 || len(r.Artifacts) > 4 {
		return ErrCorrupt
	}
	for _, id := range r.RunIDs {
		if !validID(id) {
			return ErrCorrupt
		}
	}
	for _, ref := range r.Artifacts {
		if (ArtifactKey{Session: ref.Session, Name: ref.Name}).validate() != nil || ref.Version < 1 {
			return ErrCorrupt
		}
	}
	return nil
}

func (s *Store) loadLive() (*liveCatalog, error) {
	location, err := s.path("live-sessions.json")
	if err != nil {
		return nil, fmt.Errorf("%w: locate live receipts: %w", ErrPersistence, err)
	}
	data, err := readJournalFile(location)
	if errors.Is(err, os.ErrNotExist) {
		return &liveCatalog{Version: 1, Workspace: s.workspace, Records: map[string]LiveRecord{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: read live receipts: %w", ErrPersistence, err)
	}
	var catalog liveCatalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if len(data) > 4<<20 || !validJSON(data) || decoder.Decode(&catalog) != nil || catalog.Version != 1 || catalog.Workspace != s.workspace || catalog.Records == nil || len(catalog.Records) > 128 {
		return nil, ErrCorrupt
	}
	for id, record := range catalog.Records {
		if id != record.ID || record.validate() != nil {
			return nil, ErrCorrupt
		}
	}
	return &catalog, nil
}

func (s *Store) LiveSessions(ctx context.Context, projectID string) ([]LiveRecord, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	if !validID(projectID) {
		return nil, fmt.Errorf("live session project identity is required")
	}
	catalog, err := s.loadLive()
	if err != nil {
		return nil, err
	}
	result := []LiveRecord{}
	for _, record := range catalog.Records {
		if record.ProjectID == projectID {
			result = append(result, cloneLiveRecord(record))
		}
	}
	slices.SortFunc(result, func(a, b LiveRecord) int {
		if order := b.CreatedAt.Compare(a.CreatedAt); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return result, context.Cause(ctx)
}

func cloneLiveRecord(record LiveRecord) LiveRecord {
	record.Artifacts = slices.Clone(record.Artifacts)
	record.RunIDs = slices.Clone(record.RunIDs)
	record.Usage = record.Usage.Clone()
	return record
}

// SaveLiveSession records acknowledged or uncertain effects independently of
// current source authority. The controller owns admission; receipts survive
// cancellation and never authorize another connection after restart.
func (s *Store) SaveLiveSession(ctx context.Context, record LiveRecord, expected int64) (result LiveRecord, err error) {
	record = cloneLiveRecord(record)
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	lock, err := s.lock(ctx)
	if err != nil {
		return result, persistenceOperationError(parent, "lock live receipts", err)
	}
	defer func() {
		if closeErr := lock.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("%w: unlock live receipts: %w", ErrPersistence, closeErr))
		}
	}()
	catalog, err := s.loadLive()
	if err != nil {
		return result, err
	}
	previous, exists := catalog.Records[record.ID]
	if err := admitLiveReceipt(previous, record, exists, expected, len(catalog.Records)); err != nil {
		return result, err
	}
	record.Version = expected + 1
	record.UpdatedAt = time.Now().UTC()
	if err := record.validate(); err != nil {
		return result, err
	}
	catalog.Records[record.ID] = record
	data, err := json.Marshal(catalog)
	if err != nil {
		return result, fmt.Errorf("encode live receipts: %w", err)
	}
	if len(data) > 4<<20 {
		return result, fmt.Errorf("live receipt catalog exceeds 4 MiB")
	}
	location, err := s.path("live-sessions.json")
	if err != nil {
		return result, err
	}
	if err := s.write(location, data, 0600, func() error { return context.Cause(ctx) }); err != nil {
		return result, persistenceOperationError(parent, "save live receipt", err)
	}
	return cloneLiveRecord(record), nil
}

func admitLiveReceipt(previous, record LiveRecord, exists bool, expected int64, count int) error {
	if exists && (previous.Version != expected || !sameLiveIdentity(previous, record) || !monotonicLive(previous, record)) || !exists && expected != 0 {
		return ErrLiveConflict
	}
	if !exists && count >= 128 {
		return fmt.Errorf("live receipt catalog has reached its 128-session limit")
	}
	return nil
}

func sameLiveIdentity(a, b LiveRecord) bool {
	return a.ID == b.ID && a.ProjectID == b.ProjectID && a.ProjectRevision == b.ProjectRevision && a.Policy == b.Policy && a.Configuration == b.Configuration && a.SettingsVersion == b.SettingsVersion && a.Model == b.Model && a.ProviderOrigin == b.ProviderOrigin && a.CreatedAt.Equal(b.CreatedAt) && a.ExpiresAt.Equal(b.ExpiresAt)
}

func monotonicLive(a, b LiveRecord) bool {
	return b.ControlRevision >= a.ControlRevision && b.Connections >= a.Connections && b.Turns >= a.Turns && b.SentBytes >= a.SentBytes && b.ReceivedBytes >= a.ReceivedBytes && b.ReportedInput >= a.ReportedInput && b.ReportedOutput >= a.ReportedOutput && monotonicUsage(a.Usage, b.Usage)
}
