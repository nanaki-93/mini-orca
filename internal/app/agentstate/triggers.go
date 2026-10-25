package agentstate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

var ErrTriggerConflict = errors.New("ambient analysis state changed; refresh before another control")

type TriggerOptions struct {
	Mode            string   `json:"mode"`
	IntervalSeconds int      `json:"interval_seconds"`
	Paths           []string `json:"paths"`
}

func (o TriggerOptions) Validate() error {
	if o.Mode == "disabled" {
		if len(o.Paths) != 0 || o.IntervalSeconds != 0 {
			return fmt.Errorf("disabled watches cannot select paths or an interval")
		}
		return nil
	}
	if o.Mode != "changes" && o.Mode != "interval" || len(o.Paths) < 1 || len(o.Paths) > 8 {
		return fmt.Errorf("watch changes or an interval for one to eight selected paths")
	}
	if o.Mode == "changes" && o.IntervalSeconds != 0 || o.Mode == "interval" && (o.IntervalSeconds < 60 || o.IntervalSeconds > 86400) {
		return fmt.Errorf("interval watches require 60–86400 seconds")
	}
	return validateTriggerPaths(o.Paths)
}

func validateTriggerPaths(paths []string) error {
	for i, path := range paths {
		if !fs.ValidPath(path) || path == "." || len(path) > 512 || strings.ContainsAny(path, "\\:") || strings.IndexFunc(path, unicode.IsControl) >= 0 || slices.Contains(paths[:i], path) {
			return fmt.Errorf("watch paths must be distinct canonical project-relative files")
		}
	}
	return nil
}

type TriggerAllowance struct {
	Runs         int   `json:"runs"`
	ModelCalls   int   `json:"model_calls"`
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	Seconds      int   `json:"seconds"`
}

func (a TriggerAllowance) Within(limit TriggerAllowance) bool {
	return a.Runs >= 0 && a.Runs <= limit.Runs && a.ModelCalls >= 0 && a.ModelCalls <= limit.ModelCalls && a.InputTokens >= 0 && a.InputTokens <= limit.InputTokens && a.OutputTokens >= 0 && a.OutputTokens <= limit.OutputTokens && a.Seconds >= 0 && a.Seconds <= limit.Seconds
}

func (a TriggerAllowance) Add(b TriggerAllowance) TriggerAllowance {
	return TriggerAllowance{Runs: a.Runs + b.Runs, ModelCalls: a.ModelCalls + b.ModelCalls, InputTokens: a.InputTokens + b.InputTokens, OutputTokens: a.OutputTokens + b.OutputTokens, Seconds: a.Seconds + b.Seconds}
}

type TriggerWindow struct {
	StartedAt time.Time        `json:"started_at"`
	EndsAt    time.Time        `json:"ends_at"`
	Limits    TriggerAllowance `json:"limits"`
	Charged   TriggerAllowance `json:"charged"`
}

type TriggerGrant struct {
	ID              string             `json:"id"`
	ProjectID       string             `json:"project_id"`
	Configuration   string             `json:"configuration"`
	SettingsVersion int64              `json:"settings_version"`
	Policy          string             `json:"policy"`
	ScopeHash       string             `json:"scope_hash"`
	Model           string             `json:"model"`
	ProviderOrigin  string             `json:"provider_origin"`
	CreatedAt       time.Time          `json:"created_at"`
	ExpiresAt       time.Time          `json:"expires_at"`
	RunLimits       config.AgentLimits `json:"run_limits"`
}

type TriggerAttempt struct {
	ID              string           `json:"id"`
	ProjectRevision string           `json:"project_revision"`
	Path            string           `json:"path"`
	SourceHash      string           `json:"source_hash"`
	State           string           `json:"state"`
	StartedAt       time.Time        `json:"started_at"`
	FinishedAt      time.Time        `json:"finished_at"`
	Reserved        TriggerAllowance `json:"reserved"`
	Usage           BudgetUsage      `json:"usage"`
}

type TriggerRecord struct {
	Version         int64             `json:"version"`
	ControlRevision int64             `json:"control_revision"`
	ProjectID       string            `json:"project_id"`
	Options         TriggerOptions    `json:"options"`
	Grant           *TriggerGrant     `json:"grant"`
	Window          TriggerWindow     `json:"window"`
	Observed        map[string]string `json:"observed"`
	Handled         map[string]string `json:"handled"`
	ChangedAt       time.Time         `json:"changed_at"`
	NextEligibleAt  time.Time         `json:"next_eligible_at"`
	LastAttempt     *TriggerAttempt   `json:"last_attempt"`
	State           string            `json:"state"`
	Reason          string            `json:"reason"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type triggerCatalog struct {
	Schema    int           `json:"schema"`
	Workspace string        `json:"workspace"`
	Record    TriggerRecord `json:"record"`
}

func (r TriggerRecord) validate() error {
	if !validID(r.ProjectID) || r.Version < 1 || r.Version > 1<<53 || r.ControlRevision < 1 || r.ControlRevision > r.Version || r.Options.Validate() != nil || r.UpdatedAt.IsZero() {
		return ErrCorrupt
	}
	if !slices.Contains([]string{"disabled", "watching", "suggested", "running", "completed", "failed", "interrupted", "canceled", "budget_exhausted", "stale", "unavailable", "waiting", "unchanged"}, r.State) || r.Reason != "" && !validMemoryText(r.Reason, 256, false) {
		return ErrCorrupt
	}
	if r.Options.Mode == "disabled" && r.Grant != nil {
		return ErrCorrupt
	}
	for _, check := range []func() error{r.validateSnapshots, r.validateGrant, r.validateWindow, r.validateAttempt} {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

func (r TriggerRecord) validateSnapshots() error {
	for _, values := range []map[string]string{r.Observed, r.Handled} {
		if values == nil || len(values) > 8 {
			return ErrCorrupt
		}
		for path, hash := range values {
			if !slices.Contains(r.Options.Paths, path) || !validHash(hash) {
				return ErrCorrupt
			}
		}
	}
	return nil
}

func (r TriggerRecord) validateGrant() error {
	g := r.Grant
	if g == nil {
		return nil
	}
	if g.ProjectID != r.ProjectID || !validTriggerGrantIdentity(g) {
		return ErrCorrupt
	}
	if !validMemoryText(g.Model, 128, false) || !validMemoryText(g.ProviderOrigin, 512, false) || g.CreatedAt.IsZero() || !g.ExpiresAt.After(g.CreatedAt) || g.ExpiresAt.Sub(g.CreatedAt) > 24*time.Hour {
		return ErrCorrupt
	}
	if g.RunLimits.Validate() != nil || g.RunLimits.ConcurrentCalls != 1 || g.RunLimits.Repairs != 0 || g.RunLimits.ElapsedSeconds > 300 {
		return ErrCorrupt
	}
	return nil
}

func validTriggerGrantIdentity(g *TriggerGrant) bool {
	return validID(g.ID) && validHash(g.Configuration) && validHash(g.ScopeHash) && g.SettingsVersion >= 0 && g.SettingsVersion <= 1<<53 && validMemoryText(g.Policy, 128, false)
}

func (r TriggerRecord) validateWindow() error {
	w := r.Window
	if w.StartedAt.IsZero() {
		if w != (TriggerWindow{}) {
			return ErrCorrupt
		}
		return nil
	}
	maximum := TriggerAllowance{Runs: 16, ModelCalls: 64, InputTokens: 64 << 20, OutputTokens: 16 << 20, Seconds: 86400}
	if !w.EndsAt.After(w.StartedAt) || w.EndsAt.Sub(w.StartedAt) > 24*time.Hour || !w.Limits.Within(maximum) || w.Limits.Runs < 1 || !w.Charged.Within(w.Limits) {
		return ErrCorrupt
	}
	return nil
}

func (r TriggerRecord) validateAttempt() error {
	a := r.LastAttempt
	if a == nil {
		return nil
	}
	if !validID(a.ID) || !validID(a.ProjectRevision) || !fs.ValidPath(a.Path) || !validHash(a.SourceHash) || a.StartedAt.IsZero() || !validUsage(a.Usage) {
		return ErrCorrupt
	}
	maximum := TriggerAllowance{Runs: 1, ModelCalls: 64, InputTokens: 64 << 20, OutputTokens: 16 << 20, Seconds: 300}
	if !slices.Contains([]string{"running", "completed", "failed", "interrupted", "canceled"}, a.State) || a.Reserved.Runs != 1 || !a.Reserved.Within(maximum) {
		return ErrCorrupt
	}
	if a.State == "running" && !a.FinishedAt.IsZero() || a.State != "running" && a.FinishedAt.Before(a.StartedAt) {
		return ErrCorrupt
	}
	return nil
}

func cloneTriggers(r TriggerRecord) TriggerRecord {
	r.Options.Paths = slices.Clone(r.Options.Paths)
	r.Observed, r.Handled = maps.Clone(r.Observed), maps.Clone(r.Handled)
	if r.Grant != nil {
		copy := *r.Grant
		r.Grant = &copy
	}
	if r.LastAttempt != nil {
		copy := *r.LastAttempt
		copy.Usage = copy.Usage.Clone()
		r.LastAttempt = &copy
	}
	return r
}

func (s *Store) Triggers(ctx context.Context, projectID string) (TriggerRecord, error) {
	if err := context.Cause(ctx); err != nil {
		return TriggerRecord{}, err
	}
	if !validID(projectID) {
		return TriggerRecord{}, fmt.Errorf("trigger project identity required")
	}
	location, err := s.path("triggers.json")
	if err != nil {
		return TriggerRecord{}, fmt.Errorf("%w: locate ambient state: %w", ErrPersistence, err)
	}
	data, err := readJournalFile(location)
	if errors.Is(err, os.ErrNotExist) {
		return TriggerRecord{ProjectID: projectID, Options: TriggerOptions{Mode: "disabled", Paths: []string{}}, State: "disabled", Observed: map[string]string{}, Handled: map[string]string{}}, nil
	}
	if err != nil {
		return TriggerRecord{}, fmt.Errorf("%w: read ambient state: %w", ErrPersistence, err)
	}
	var catalog triggerCatalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if len(data) > 64<<10 || !validJSON(data) || decoder.Decode(&catalog) != nil || catalog.Schema != 1 || catalog.Workspace != s.workspace || catalog.Record.validate() != nil {
		return TriggerRecord{}, ErrCorrupt
	}
	if catalog.Record.ProjectID != projectID {
		return TriggerRecord{}, ErrTriggerConflict
	}
	return cloneTriggers(catalog.Record), context.Cause(ctx)
}

// Control and progress share CAS; a new grant cannot replenish an open window.
func (s *Store) SaveTriggers(ctx context.Context, record TriggerRecord, expected int64) (result TriggerRecord, err error) {
	record = cloneTriggers(record)
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	lock, err := s.lock(ctx)
	if err != nil {
		return result, persistenceOperationError(parent, "lock ambient state", err)
	}
	defer func() {
		if closeErr := lock.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("%w: unlock ambient state: %w", ErrPersistence, closeErr))
		}
	}()
	previous, err := s.Triggers(ctx, record.ProjectID)
	if err != nil {
		return result, err
	}
	if previous.Version != expected || !validTriggerTransition(previous, record) {
		return result, ErrTriggerConflict
	}
	record.Version, record.UpdatedAt = expected+1, time.Now().UTC()
	if err := record.validate(); err != nil {
		return result, err
	}
	data, err := json.Marshal(triggerCatalog{Schema: 1, Workspace: s.workspace, Record: record})
	if err != nil {
		return result, fmt.Errorf("encode ambient state: %w", err)
	}
	if len(data) > 64<<10 {
		return result, fmt.Errorf("ambient state exceeds 64 KiB")
	}
	location, err := s.path("triggers.json")
	if err != nil {
		return result, err
	}
	if err := s.write(location, data, 0600, func() error { return context.Cause(ctx) }); err != nil {
		return result, persistenceOperationError(parent, "save ambient state", err)
	}
	return cloneTriggers(record), nil
}

func validTriggerTransition(old, next TriggerRecord) bool {
	if next.ControlRevision < old.ControlRevision || next.ControlRevision > old.ControlRevision+1 {
		return false
	}
	if !sameTriggerControl(old, next) && next.ControlRevision != old.ControlRevision+1 {
		return false
	}
	if old.Window.StartedAt.IsZero() {
		return true
	}
	if next.Window.StartedAt == old.Window.StartedAt {
		return next.Window.EndsAt == old.Window.EndsAt && next.Window.Limits == old.Window.Limits && old.Window.Charged.Within(next.Window.Charged)
	}
	return !time.Now().Before(old.Window.EndsAt) && !next.Window.StartedAt.Before(old.Window.EndsAt)
}

func sameTriggerControl(a, b TriggerRecord) bool {
	if a.Options.Mode != b.Options.Mode || a.Options.IntervalSeconds != b.Options.IntervalSeconds || !slices.Equal(a.Options.Paths, b.Options.Paths) {
		return false
	}
	if a.Grant == nil || b.Grant == nil {
		return a.Grant == nil && b.Grant == nil
	}
	return *a.Grant == *b.Grant
}
