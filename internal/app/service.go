// Package app contains the daemon's configured application service.
package app

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// EffectiveModel is the actual generation profile used by the daemon.
type EffectiveModel struct {
	Scope            string   `json:"scope"`
	Profile          string   `json:"profile"`
	Model            string   `json:"model"`
	ReasoningEffort  string   `json:"reasoning_effort,omitempty"`
	ProviderOrigin   string   `json:"provider_origin"`
	RemoteProvider   bool     `json:"remote_provider"`
	Temperature      float32  `json:"temperature"`
	MaxTokens        int      `json:"max_tokens"`
	ContextMaxTokens int      `json:"context_max_tokens"`
	Skills           []string `json:"skills"`
	Timeout          string   `json:"timeout"`
	MaxRetries       int      `json:"max_retries"`
}

// ScopedModel is the non-secret API representation of one effective runtime.
type ScopedModel struct {
	Scope            string  `json:"scope"`
	Profile          string  `json:"profile"`
	Model            string  `json:"model"`
	ReasoningEffort  string  `json:"reasoning_effort,omitempty"`
	ProviderOrigin   string  `json:"provider_origin"`
	RemoteProvider   bool    `json:"remote_provider"`
	Temperature      float32 `json:"temperature"`
	MaxTokens        int     `json:"max_tokens"`
	ContextMaxTokens int     `json:"context_max_tokens"`
	Timeout          string  `json:"timeout"`
	MaxRetries       int     `json:"max_retries"`
}

// ModelCatalog preserves the legacy top-level function profile and adds the
// complete fixed-scope catalog for new clients.
type ModelCatalog struct {
	EffectiveModel
	Scopes map[string]ScopedModel `json:"scopes"`
}

type modelRuntime struct {
	profile   config.ModelProfile
	effective EffectiveModel
	client    *llm.Client
}

type modelOutput struct {
	Content string
	Model   string
}

// Service owns configured model access and the active project's generation path.
type Service struct {
	manager             *project.Manager
	analyzeRuntime      modelRuntime
	bugRuntime          modelRuntime
	functionRuntime     modelRuntime
	importTimeout       time.Duration
	analysisTimeout     time.Duration
	focusedCheckTimeout time.Duration
	retryBase           time.Duration
	retryMax            time.Duration
	analysisAll         *analysisAllController
	goScan              *goScanController
	drafts              map[string]*storedDraft
	draftAudit          []DraftAuditMetadata
	draftMu             sync.Mutex
	chatSessions        map[string]*chatSession
	chatSessionMu       sync.Mutex
}

func New(cfg *config.Config, manager *project.Manager) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("application config is required")
	}
	if manager == nil {
		return nil, fmt.Errorf("project manager is required")
	}

	profiles, err := config.ResolveModelProfiles(cfg)
	if err != nil {
		return nil, err
	}
	functionTimeout := configuredDuration(cfg.Timeouts.GenerationSeconds, cfg.Agents.Coder.TimeoutSeconds, 5*time.Minute)
	importTimeout := configuredDuration(cfg.Timeouts.ImportSeconds, 0, 5*time.Minute)
	analysisTimeout := configuredDuration(cfg.Timeouts.AnalysisSeconds, 0, 5*time.Minute)
	maxRetries := cfg.Retry.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}
	backoffBase := cfg.Retry.BackoffBase
	if backoffBase == 0 {
		backoffBase = 1000
	}
	backoffMax := cfg.Retry.BackoffMax
	if backoffMax == 0 {
		backoffMax = 30000
	}

	return &Service{
		manager:             manager,
		analyzeRuntime:      newModelRuntime(profiles.Analyze, importTimeout, maxRetries),
		bugRuntime:          newModelRuntime(profiles.Bug, analysisTimeout, maxRetries),
		functionRuntime:     newModelRuntime(profiles.Function, functionTimeout, maxRetries),
		importTimeout:       importTimeout,
		analysisTimeout:     analysisTimeout,
		focusedCheckTimeout: configuredDuration(cfg.Timeouts.FocusedCheckSeconds, 0, time.Minute),
		retryBase:           time.Duration(backoffBase) * time.Millisecond,
		retryMax:            time.Duration(backoffMax) * time.Millisecond,
		analysisAll:         newAnalysisAllController(),
		goScan:              newGoScanController(),
		drafts:              make(map[string]*storedDraft),
		chatSessions:        make(map[string]*chatSession),
	}, nil
}

func newModelRuntime(profile config.ModelProfile, timeout time.Duration, maxRetries int) modelRuntime {
	runtime := modelRuntime{
		profile: profile,
		client:  llm.NewClientWithAPIBaseAndReasoningEffort(profile.APIBaseURL, profile.APIKey, profile.Model, profile.Temperature, profile.MaxTokens, profile.ReasoningEffort),
		effective: EffectiveModel{
			Scope: string(profile.Scope), Profile: string(profile.Scope), Model: profile.Model,
			ReasoningEffort: profile.ReasoningEffort,
			ProviderOrigin:  providerOrigin(profile.APIBaseURL), RemoteProvider: !isLoopbackURL(profile.APIBaseURL),
			Temperature: profile.Temperature, MaxTokens: profile.MaxTokens, ContextMaxTokens: profile.ContextMaxTokens,
			Timeout: timeout.String(), MaxRetries: maxRetries,
		},
	}
	return runtime
}

func configuredDuration(primarySeconds, fallbackSeconds int, defaultValue time.Duration) time.Duration {
	seconds := primarySeconds
	if seconds == 0 {
		seconds = fallbackSeconds
	}
	if seconds <= 0 {
		return defaultValue
	}
	return time.Duration(seconds) * time.Second
}

// ValidateMutableRequest rejects candidates based on a different project or file state.
func (s *Service) ValidateMutableRequest(id, revision, targetFile, baseFileHash string) error {
	return s.manager.ValidateMutableRequest(id, revision, targetFile, baseFileHash)
}

// RecordActivity persists source-free activity for the accepted project revision.
func (s *Service) RecordActivity(id, revision string, activity project.Activity) error {
	return s.manager.RecordActivityFor(id, revision, activity)
}

// Activity returns only the active project's durable activity history.
func (s *Service) Activity() ([]project.Activity, error) {
	return s.manager.Activity()
}

// AnalyzeProject runs import analysis under its own deadline.
func (s *Service) AnalyzeProject(ctx context.Context, root string) (*project.Analysis, error) {
	timed, cancel := context.WithTimeout(ctx, s.importTimeout)
	defer cancel()
	analysis, err := project.NewAnalyzerWithProvenance(s.analyzeRuntime.client, s.analyzeRuntime.profile.Model, string(config.AnalyzeModelScope), s.analyzeRuntime.effective.ProviderOrigin, s.analyzeRuntime.effective.ReasoningEffort).Analyze(timed, root)
	if timed.Err() != nil {
		return nil, timed.Err()
	}
	return analysis, err
}

// RestoreProject reopens a previously imported project from local persisted
// analysis without making a model request.
func (s *Service) RestoreProject(root string) (*project.Analysis, error) {
	return project.NewAnalyzerWithProvenance(nil, s.analyzeRuntime.profile.Model, string(config.AnalyzeModelScope), s.analyzeRuntime.effective.ProviderOrigin, s.analyzeRuntime.effective.ReasoningEffort).Restore(root)
}

func (s *Service) EffectiveModel() EffectiveModel {
	profile := s.functionRuntime.effective
	profile.Skills = append([]string(nil), s.functionRuntime.effective.Skills...)
	return profile
}

// EffectiveModels returns safe metadata for every configured prompt scope.
func (s *Service) EffectiveModels() []EffectiveModel {
	profiles := []EffectiveModel{s.analyzeRuntime.effective, s.bugRuntime.effective, s.functionRuntime.effective}
	for index := range profiles {
		profiles[index].Skills = append([]string(nil), profiles[index].Skills...)
	}
	return profiles
}

// CurrentModelCatalog returns only the safe effective metadata needed for
// model destination display and remote-confirmation decisions.
func (s *Service) CurrentModelCatalog() ModelCatalog {
	catalog := ModelCatalog{EffectiveModel: s.EffectiveModel(), Scopes: make(map[string]ScopedModel, 3)}
	for _, profile := range s.EffectiveModels() {
		catalog.Scopes[profile.Scope] = scopedModel(profile)
	}
	return catalog
}

func scopedModel(profile EffectiveModel) ScopedModel {
	return ScopedModel{
		Scope: profile.Scope, Profile: profile.Profile, Model: profile.Model, ReasoningEffort: profile.ReasoningEffort, ProviderOrigin: profile.ProviderOrigin,
		RemoteProvider: profile.RemoteProvider, Temperature: profile.Temperature, MaxTokens: profile.MaxTokens,
		ContextMaxTokens: profile.ContextMaxTokens, Timeout: profile.Timeout, MaxRetries: profile.MaxRetries,
	}
}

// RequireRemoteConfirmation prevents accidental prompt delivery to the actual
// selected scope's non-local provider.
func (s *Service) RequireRemoteConfirmation(scope config.ModelScope, confirmed bool) error {
	runtime, ok := s.modelRuntimeForScope(scope)
	if !ok {
		return fmt.Errorf("unknown model scope %q", scope)
	}
	if runtime.effective.RemoteProvider && !confirmed {
		return fmt.Errorf("remote %s provider requires explicit confirmation", scope)
	}
	return nil
}

func (s *Service) modelRuntimeForScope(scope config.ModelScope) (modelRuntime, bool) {
	switch scope {
	case config.AnalyzeModelScope:
		return s.analyzeRuntime, true
	case config.BugModelScope:
		return s.bugRuntime, true
	case config.FunctionModelScope:
		return s.functionRuntime, true
	default:
		return modelRuntime{}, false
	}
}

// ContextManifest previews the safe prompt context for one selected file.
func (s *Service) ContextManifest(targetFile string) (project.ContextManifest, error) {
	_, manifest, err := project.NewContextBuilder().BuildWithManifest(s.manager.Root(), targetFile)
	return s.contextManifestForRuntime(manifest, s.functionRuntime), err
}

// AnalysisContextManifest previews the one-file context used for semantic analysis.
func (s *Service) AnalysisContextManifest(targetFile string) (project.ContextManifest, error) {
	indexedFile, err := s.manager.IndexedFile(targetFile)
	if err != nil {
		return project.ContextManifest{}, err
	}
	return s.contextManifestForRuntime(semanticManifest(*indexedFile), s.bugRuntime), nil
}

func (s *Service) contextManifestForRuntime(manifest project.ContextManifest, runtime modelRuntime) project.ContextManifest {
	manifest.Scope = runtime.effective.Scope
	manifest.Model = runtime.effective.Model
	manifest.ProviderOrigin = runtime.effective.ProviderOrigin
	manifest.RemoteProvider = runtime.effective.RemoteProvider
	return manifest
}

func isLoopbackURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.Trim(parsed.Hostname(), "[]")
	if host == "localhost" || host == "::1" {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func (s *Service) retry(ctx context.Context, runtime modelRuntime, messages []llm.ChatMessage) (modelOutput, error) {
	var lastErr error
	for attempt := 0; attempt <= runtime.effective.MaxRetries; attempt++ {
		result, err := runtime.execute(ctx, messages)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if ctx.Err() != nil || attempt == runtime.effective.MaxRetries {
			break
		}
		wait := s.retryBase << attempt
		if wait > s.retryMax {
			wait = s.retryMax
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return modelOutput{}, ctx.Err()
		case <-timer.C:
		}
	}
	if ctx.Err() != nil {
		return modelOutput{}, ctx.Err()
	}
	return modelOutput{}, lastErr
}

func (r modelRuntime) execute(ctx context.Context, messages []llm.ChatMessage) (modelOutput, error) {
	if r.client == nil {
		return modelOutput{}, fmt.Errorf("model client is not configured")
	}
	if len(messages) == 0 || strings.TrimSpace(messages[0].Content) == "" {
		return modelOutput{}, fmt.Errorf("model request is required")
	}
	response, err := r.client.Chat(ctx, messages)
	if err != nil {
		return modelOutput{}, fmt.Errorf("model request failed: %w", err)
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return modelOutput{}, fmt.Errorf("model response content is required")
	}
	return modelOutput{Content: response.Choices[0].Message.Content, Model: response.Model}, nil
}

func providerOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func duration(value string) time.Duration {
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 5 * time.Minute
	}
	return parsed
}
