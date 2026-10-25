package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

var ErrLiveUnavailable = errors.New("live provider exchange unavailable")

type LiveBinding struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	Configuration   string `json:"configuration"`
	SettingsVersion int64  `json:"settings_version"`
	Model           string `json:"model"`
	ProviderOrigin  string `json:"provider_origin"`
}

type LiveOpenRequest struct {
	LiveBinding
	ConfirmConnect        bool `json:"confirm_connect"`
	ConfirmRemoteProvider bool `json:"confirm_remote_provider"`
}

type LiveControl struct {
	LiveBinding
	ControlRevision       int64                `json:"control_revision"`
	Action                string               `json:"action"`
	Text                  string               `json:"text,omitempty"`
	Attachment            *AttachmentReference `json:"attachment,omitempty"`
	ConfirmConnect        bool                 `json:"confirm_connect"`
	ConfirmSourceSharing  bool                 `json:"confirm_source_sharing"`
	ConfirmRemoteProvider bool                 `json:"confirm_remote_provider"`
}

type LiveSession struct {
	agentstate.LiveRecord
	Owned            bool            `json:"owned"`
	Stale            bool            `json:"stale"`
	CanReconnect     bool            `json:"can_reconnect"`
	RetentionFailed  bool            `json:"retention_failed"`
	OutputArtifacts  []AgentArtifact `json:"output_artifacts"`
	MissingArtifacts int             `json:"missing_artifacts"`
}

type LiveOverview struct {
	LiveBinding
	State                 string             `json:"state"`
	Destination           EffectiveModel     `json:"destination"`
	Limits                config.AgentLimits `json:"limits"`
	LifetimeSeconds       int                `json:"lifetime_seconds"`
	MaxConnections        int                `json:"max_connections"`
	MaxTurnsPerConnection int                `json:"max_turns_per_connection"`
	MaxInputBytes         int                `json:"max_input_bytes"`
	MaxOutputBytes        int                `json:"max_output_bytes"`
	Sessions              []LiveSession      `json:"sessions"`
}

func (s *Service) liveRuntime() modelRuntime {
	profile := s.runtimes.function.profile
	profile.Model = s.agentRuntime.LiveModel
	profile.ReasoningEffort = ""
	runtime := newModelRuntime(profile, agentruntime.LiveLifetime, 0)
	runtime.effective.Temperature = nil
	runtime.effective.MaxTokens = nil
	return runtime
}

func (s *Service) LiveSessions(ctx context.Context, query AgentRunQuery) (*LiveOverview, error) {
	root := s.manager.Root()
	settings, err := s.AgentSettings(ctx, query)
	if err != nil {
		return nil, err
	}
	runtime := s.liveRuntime()
	result := &LiveOverview{LiveBinding: LiveBinding{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Configuration: settings.Configuration, SettingsVersion: settings.Version, Model: runtime.effective.Model, ProviderOrigin: runtime.effective.ProviderOrigin}, State: "available", Destination: runtime.effective, Limits: settings.Limits, LifetimeSeconds: 300, MaxConnections: 2, MaxTurnsPerConnection: 8, MaxInputBytes: agentruntime.LiveMaxInputBytes, MaxOutputBytes: agentruntime.LiveMaxOutputBytes, Sessions: []LiveSession{}}
	switch {
	case s.agentRuntime.LiveModel == "":
		result.State = "disabled"
	case !llm.SupportsLiveSessions(runtime.profile):
		result.State = "unsupported_profile"
	case settings.State == "stale":
		result.State = "stale_settings"
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, artifactLibraryError(err)
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return nil, artifactLibraryError(err)
	}
	records, err := store.LiveSessions(ctx, query.ProjectID)
	if err != nil {
		return nil, artifactLibraryError(err)
	}
	for _, record := range records {
		view, err := s.liveView(ctx, root, query, record, result.LiveBinding, policy.Version(), false)
		if err != nil {
			return nil, err
		}
		result.Sessions = append(result.Sessions, *view)
	}
	return result, s.artifactLibraryScope(ctx, query, root)
}

func (s *Service) LiveSessionDetail(ctx context.Context, query AgentRunQuery, id string) (*LiveSession, error) {
	overview, err := s.LiveSessions(ctx, query)
	if err != nil {
		return nil, err
	}
	for _, view := range overview.Sessions {
		if view.ID == id {
			root := s.manager.Root()
			policy, err := project.NewContextPolicy(root)
			if err != nil {
				return nil, artifactLibraryError(err)
			}
			return s.liveView(ctx, root, query, view.LiveRecord, overview.LiveBinding, policy.Version(), true)
		}
	}
	return nil, agentstate.ErrNotFound
}

func (s *Service) liveView(ctx context.Context, root string, query AgentRunQuery, record agentstate.LiveRecord, binding LiveBinding, policy string, outputs bool) (*LiveSession, error) {
	view := &LiveSession{LiveRecord: record, OutputArtifacts: []AgentArtifact{}}
	disconnected := false
	if owner := s.live.owner(root, record.ID); owner != nil {
		owner.mu.Lock()
		view.LiveRecord = owner.currentLocked()
		view.Owned = owner.ctx.Err() == nil
		view.RetentionFailed = owner.failure != nil
		disconnected = owner.connection == nil
		owner.mu.Unlock()
	}
	view.Stale = liveRecordBinding(view.LiveRecord) != binding || view.Policy != policy
	if !view.Owned && slices.Contains([]string{"connecting", "connected", "disconnected", "go_away"}, view.State) {
		view.State = "interrupted"
	}
	view.CanReconnect = disconnected && view.Owned && !view.Stale && view.Connections < 2 && view.ExpiresAt.After(time.Now()) && slices.Contains([]string{"failed", "disconnected", "go_away"}, view.State) && !view.RetentionFailed
	if outputs {
		if err := s.liveOutputs(ctx, query, root, view); err != nil {
			return nil, err
		}
	}
	return view, nil
}

func (s *Service) liveOutputs(ctx context.Context, query AgentRunQuery, root string, view *LiveSession) error {
	store, entries, err := s.artifactInventory(ctx, query, root)
	if err != nil {
		return err
	}
	for _, ref := range view.Artifacts {
		found := false
		for _, entry := range entries {
			if entry.Artifact.Key.Session == ref.Session && entry.Artifact.Key.Name == ref.Name && entry.Artifact.Version == ref.Version {
				if output, owned := artifactLibraryView(store, entry, query); owned {
					view.OutputArtifacts = append(view.OutputArtifacts, output)
					found = true
				}
				break
			}
		}
		if !found {
			view.MissingArtifacts++
		}
	}
	return nil
}

func liveRecordBinding(record agentstate.LiveRecord) LiveBinding {
	return LiveBinding{ProjectID: record.ProjectID, ProjectRevision: record.ProjectRevision, Configuration: record.Configuration, SettingsVersion: record.SettingsVersion, Model: record.Model, ProviderOrigin: record.ProviderOrigin}
}

func (s *Service) validateLiveBinding(ctx context.Context, root, policy string, binding LiveBinding) error {
	query := AgentRunQuery{ProjectID: binding.ProjectID, ProjectRevision: binding.ProjectRevision}
	if err := s.artifactLibraryScope(ctx, query, root); err != nil {
		return err
	}
	settings, err := s.AgentSettings(ctx, query)
	if err != nil {
		return err
	}
	runtime := s.liveRuntime()
	if settings.State == "stale" || settings.Configuration != binding.Configuration || settings.Version != binding.SettingsVersion || runtime.effective.Model != binding.Model || runtime.effective.ProviderOrigin != binding.ProviderOrigin {
		return fmt.Errorf("%w: inspect current live destination and settings", project.ErrRevisionConflict)
	}
	current, err := project.NewContextPolicy(root)
	if err != nil {
		return err
	}
	if current.Version() != policy {
		return fmt.Errorf("%w: live context policy changed", project.ErrRevisionConflict)
	}
	return nil
}
