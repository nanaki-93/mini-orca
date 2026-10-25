package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type TriggerBinding struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	Configuration   string `json:"configuration"`
	SettingsVersion int64  `json:"settings_version"`
	Policy          string `json:"policy"`
	Model           string `json:"model"`
	ProviderOrigin  string `json:"provider_origin"`
}

type AgentTriggers struct {
	TriggerBinding
	State       string                   `json:"state"`
	Owned       bool                     `json:"owned"`
	Fault       bool                     `json:"fault"`
	ScopeHash   string                   `json:"scope_hash"`
	Record      agentstate.TriggerRecord `json:"record"`
	Ceilings    config.AgentTriggers     `json:"ceilings"`
	RunLimits   config.AgentLimits       `json:"run_limits"`
	Destination EffectiveModel           `json:"destination"`
}

type TriggerControl struct {
	TriggerBinding
	ControlRevision       int64                      `json:"control_revision"`
	Action                string                     `json:"action"`
	Confirm               bool                       `json:"confirm"`
	Options               *agentstate.TriggerOptions `json:"options,omitempty"`
	ScopeHash             string                     `json:"scope_hash,omitempty"`
	GrantSeconds          int                        `json:"grant_seconds,omitempty"`
	ConfirmSourceSharing  bool                       `json:"confirm_source_sharing,omitempty"`
	ConfirmRemoteProvider bool                       `json:"confirm_remote_provider,omitempty"`
}

func (s *Service) AgentTriggers(ctx context.Context, query AgentRunQuery) (*AgentTriggers, error) {
	root := s.manager.Root()
	if err := s.integrationScope(ctx, query, root); err != nil {
		return nil, err
	}
	settings, err := s.AgentSettings(ctx, query)
	if err != nil {
		return nil, err
	}
	store, _, err := s.agentJournal(root)
	if err != nil {
		return nil, err
	}
	record, err := store.Triggers(ctx, query.ProjectID)
	if err != nil {
		return nil, err
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	runtime := s.semanticCacheRuntime(analysis)
	configuration, err := analysisFingerprint(struct {
		Settings string
		Triggers config.AgentTriggers
		Provider config.ModelProfile
	}{settings.Configuration, s.agentRuntime.Triggers, runtime.profile})
	if err != nil {
		return nil, err
	}
	scopeHash, err := analysisFingerprint(record.Options)
	if err != nil {
		return nil, err
	}
	model := runtime.effective
	view := &AgentTriggers{TriggerBinding: TriggerBinding{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Configuration: configuration, SettingsVersion: settings.Version, Policy: policy.Version(), Model: model.Model, ProviderOrigin: model.ProviderOrigin}, Record: record, Ceilings: s.agentRuntime.Triggers, RunLimits: s.agentRuntime.Triggers.RunLimits(settings.Limits), ScopeHash: scopeHash, Destination: model}
	view.Owned, view.Fault = s.triggerOwnerState(root, record.ControlRevision)
	view.State = triggerViewState(view, settings.State, time.Now())
	return view, s.integrationScope(ctx, query, root)
}

func triggerViewState(view *AgentTriggers, settings string, now time.Time) string {
	switch {
	case view.Fault:
		return "failed"
	case view.Record.Options.Mode == "disabled":
		return "disabled"
	case settings == "stale":
		return "stale_settings"
	case !view.Owned:
		return "interrupted"
	case view.Record.Grant == nil:
		return "watch_only"
	case !view.Record.Grant.ExpiresAt.After(now):
		return "grant_expired"
	case !triggerGrantMatches(view.Record.Grant, view.TriggerBinding, view.ScopeHash):
		return "stale_grant"
	default:
		return "granted"
	}
}

func triggerGrantMatches(grant *agentstate.TriggerGrant, binding TriggerBinding, scopeHash string) bool {
	return grant != nil && grant.ProjectID == binding.ProjectID && grant.Configuration == binding.Configuration && grant.SettingsVersion == binding.SettingsVersion && grant.Policy == binding.Policy && grant.Model == binding.Model && grant.ProviderOrigin == binding.ProviderOrigin && grant.ScopeHash == scopeHash
}

func (s *Service) ControlAgentTriggers(ctx context.Context, request TriggerControl) (*AgentTriggers, error) {
	if err := validateTriggerControl(request); err != nil {
		return nil, err
	}
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	query := AgentRunQuery{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}
	view, err := s.AgentTriggers(ctx, query)
	if err != nil {
		return nil, err
	}
	if err := admitTriggerControl(view, request); err != nil {
		return nil, err
	}
	if err := s.checkTriggerControlSource(ctx, view, request); err != nil {
		return nil, err
	}
	if err := s.stopTriggers(ctx); err != nil {
		return nil, err
	}
	store, _, err := s.agentJournal(s.manager.Root())
	if err != nil {
		return nil, err
	}
	record, err := store.Triggers(ctx, query.ProjectID)
	if err != nil {
		return nil, err
	}
	if record.ControlRevision != request.ControlRevision {
		return nil, agentstate.ErrTriggerConflict
	}
	if err := s.changeTriggerControl(ctx, &record, view, request); err != nil {
		return nil, err
	}
	record, err = store.SaveTriggers(ctx, record, record.Version)
	if err != nil {
		return nil, err
	}
	if record.Options.Mode != "disabled" {
		s.startTriggerOwner(s.manager.Root(), store, record)
	}
	s.notifyEvent("agent_triggers")
	return s.AgentTriggers(ctx, query)
}

func (s *Service) checkTriggerControlSource(ctx context.Context, view *AgentTriggers, request TriggerControl) error {
	if request.Action == "grant" && view.Destination.RemoteProvider && !request.ConfirmRemoteProvider {
		return fmt.Errorf("automatic analysis requires explicit confirmation for its current remote provider")
	}
	if request.Options != nil {
		_, err := s.captureTriggerSources(ctx, s.manager.Root(), request.Options.Paths, true)
		return err
	}
	return nil
}

func validateTriggerControl(request TriggerControl) error {
	if !request.Confirm || !slices.Contains([]string{"watch", "grant", "revoke", "disable"}, request.Action) || request.ControlRevision < 0 {
		return fmt.Errorf("ambient controls require an exact current revision and explicit confirmation")
	}
	if request.Action == "watch" {
		if request.Options == nil || request.Options.Mode == "disabled" {
			return fmt.Errorf("watch requires explicit paths and changes or interval mode")
		}
		return request.Options.Validate()
	}
	if request.Options != nil {
		return fmt.Errorf("only watch accepts replacement paths or timing")
	}
	if request.Action == "grant" && (!request.ConfirmSourceSharing || request.ScopeHash == "" || request.GrantSeconds < 60) {
		return fmt.Errorf("provider permission requires exact scope, bounded expiry and explicit source sharing")
	}
	return nil
}

func admitTriggerControl(view *AgentTriggers, request TriggerControl) error {
	if request.ProjectID != view.ProjectID || request.ControlRevision != view.Record.ControlRevision {
		return agentstate.ErrTriggerConflict
	}
	if request.Action == "disable" || request.Action == "revoke" {
		return nil
	}
	if request.TriggerBinding != view.TriggerBinding {
		return project.ErrRevisionConflict
	}
	if request.Action == "grant" && (view.Record.Options.Mode == "disabled" || request.ScopeHash != view.ScopeHash || request.GrantSeconds > view.Ceilings.GrantSeconds || view.State == "stale_settings" || view.RunLimits.Validate() != nil) {
		return fmt.Errorf("refresh the enabled watch, current settings, exact paths and grant lifetime")
	}
	return nil
}

func (s *Service) changeTriggerControl(ctx context.Context, record *agentstate.TriggerRecord, view *AgentTriggers, request TriggerControl) error {
	now := time.Now().UTC()
	settleInterruptedTrigger(record, now)
	record.ControlRevision++
	record.Grant = nil
	switch request.Action {
	case "disable":
		record.Options = agentstate.TriggerOptions{Mode: "disabled", Paths: []string{}}
		record.Observed, record.Handled = map[string]string{}, map[string]string{}
		record.State, record.Reason, record.NextEligibleAt = "disabled", "Watch and automatic provider work disabled.", time.Time{}
		return nil
	case "watch":
		record.Options = *request.Options
		record.Options.Paths = slices.Clone(request.Options.Paths)
		slices.Sort(record.Options.Paths)
	case "grant":
		record.Grant = &agentstate.TriggerGrant{ID: rand.Text(), ProjectID: view.ProjectID, Configuration: view.Configuration, SettingsVersion: view.SettingsVersion, Policy: view.Policy, ScopeHash: view.ScopeHash, Model: view.TriggerBinding.Model, ProviderOrigin: view.ProviderOrigin, CreatedAt: now, ExpiresAt: now.Add(time.Duration(request.GrantSeconds) * time.Second), RunLimits: view.RunLimits}
	}
	if record.Options.Mode == "disabled" {
		record.State, record.Reason = "disabled", "Automatic provider permission revoked."
		return nil
	}
	observed, err := s.captureTriggerSources(ctx, s.manager.Root(), record.Options.Paths, true)
	if err != nil {
		return err
	}
	resetTriggerBaseline(record, observed, now, s.agentRuntime.Triggers)
	if request.Action == "grant" && record.Options.Mode == "interval" {
		record.Handled = map[string]string{}
	}
	return nil
}

func resetTriggerBaseline(record *agentstate.TriggerRecord, observed map[string]string, now time.Time, limits config.AgentTriggers) {
	record.Observed, record.Handled, record.ChangedAt = maps.Clone(observed), maps.Clone(observed), now
	delay := limits.QuietSeconds
	if record.Options.Mode == "interval" {
		delay = max(delay, record.Options.IntervalSeconds)
	}
	record.NextEligibleAt = now.Add(time.Duration(delay) * time.Second)
	record.State, record.Reason = "watching", "Fresh baseline recorded; no missed inputs will be replayed."
}

func settleInterruptedTrigger(record *agentstate.TriggerRecord, now time.Time) {
	if record.LastAttempt != nil && record.LastAttempt.State == "running" {
		record.LastAttempt.State, record.LastAttempt.FinishedAt = "interrupted", now
	}
}
