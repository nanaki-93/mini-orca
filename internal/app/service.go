// Package app contains the daemon's configured application service.
package app

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/agent"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// EffectiveModel is the actual generation profile used by the daemon.
type EffectiveModel struct {
	Profile     string   `json:"profile"`
	Model       string   `json:"model"`
	Temperature float32  `json:"temperature"`
	MaxTokens   int      `json:"max_tokens"`
	Skills      []string `json:"skills"`
	Timeout     string   `json:"timeout"`
	MaxRetries  int      `json:"max_retries"`
}

// Service owns configured model access and the active project's generation path.
type Service struct {
	manager             *project.Manager
	analysisClient      *llm.Client
	coder               *agent.CoderAgent
	profile             EffectiveModel
	importTimeout       time.Duration
	analysisTimeout     time.Duration
	focusedCheckTimeout time.Duration
	retryBase           time.Duration
	retryMax            time.Duration
	remoteProvider      bool
}

func New(cfg *config.Config, manager *project.Manager) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("application config is required")
	}
	if manager == nil {
		return nil, fmt.Errorf("project manager is required")
	}

	model := cfg.Agents.Coder.Model
	if model == "" {
		model = cfg.LLM.Model
	}
	analysisClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.Temperature, cfg.LLM.MaxTokens)
	coderClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, model, cfg.LLM.Temperature, cfg.LLM.MaxTokens)
	coder := agent.NewCoderAgent(coderClient)
	coder.SetSkills(append([]string(nil), cfg.Agents.Coder.Skills...))

	timeout := configuredDuration(cfg.Timeouts.GenerationSeconds, cfg.Agents.Coder.TimeoutSeconds, 5*time.Minute)
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
		manager:        manager,
		analysisClient: analysisClient,
		coder:          coder,
		profile: EffectiveModel{
			Profile: "coder", Model: model, Temperature: cfg.LLM.Temperature, MaxTokens: cfg.LLM.MaxTokens,
			Skills: append([]string(nil), cfg.Agents.Coder.Skills...), Timeout: timeout.String(), MaxRetries: maxRetries,
		},
		importTimeout:       configuredDuration(cfg.Timeouts.ImportSeconds, 0, 5*time.Minute),
		analysisTimeout:     configuredDuration(cfg.Timeouts.AnalysisSeconds, 0, 5*time.Minute),
		focusedCheckTimeout: configuredDuration(cfg.Timeouts.FocusedCheckSeconds, 0, time.Minute),
		retryBase:           time.Duration(backoffBase) * time.Millisecond,
		retryMax:            time.Duration(backoffMax) * time.Millisecond,
		remoteProvider:      !isLoopbackURL(cfg.LLM.BaseURL),
	}, nil
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

func (s *Service) AnalysisClient() *llm.Client { return s.analysisClient }

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
	analysis, err := project.NewAnalyzer(s.analysisClient).Analyze(timed, root)
	if timed.Err() != nil {
		return nil, timed.Err()
	}
	return analysis, err
}

func (s *Service) EffectiveModel() EffectiveModel {
	profile := s.profile
	profile.Skills = append([]string(nil), s.profile.Skills...)
	return profile
}

// RequireRemoteConfirmation prevents accidental prompt delivery to a non-local provider.
func (s *Service) RequireRemoteConfirmation(confirmed bool) error {
	if s.remoteProvider && !confirmed {
		return fmt.Errorf("remote LLM provider requires explicit confirmation")
	}
	return nil
}

// ContextManifest previews the safe prompt context for one selected file.
func (s *Service) ContextManifest(targetFile string) (project.ContextManifest, error) {
	_, manifest, err := project.NewContextBuilder().BuildWithManifest(s.manager.Root(), targetFile)
	return manifest, err
}

// Generate produces a preview only. It never writes to the active project.
func (s *Service) Generate(ctx context.Context, userPrompt, targetFile, targetSymbol string, confirmRemoteProvider bool) (*agent.Result, error) {
	if err := s.RequireRemoteConfirmation(confirmRemoteProvider); err != nil {
		return nil, err
	}
	root := s.manager.Root()
	fileInfo, err := project.GetFileInfo(root, targetFile)
	if err != nil {
		return nil, err
	}
	if fileInfo.Binary {
		return nil, fmt.Errorf("target file must be a text source file")
	}
	projectContext, _, err := project.NewContextBuilder().BuildWithManifest(root, targetFile)
	if err != nil {
		return nil, fmt.Errorf("build project context: %w", err)
	}
	input, err := agent.AtomicCoderInput(userPrompt, projectContext, targetFile, targetSymbol)
	if err != nil {
		return nil, err
	}

	timed, cancel := context.WithTimeout(ctx, duration(s.profile.Timeout))
	defer cancel()
	result, err := s.retry(timed, input)
	if timed.Err() != nil {
		return nil, timed.Err()
	}
	return result, err
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

func (s *Service) retry(ctx context.Context, input string) (*agent.Result, error) {
	var lastErr error
	for attempt := 0; attempt <= s.profile.MaxRetries; attempt++ {
		result, err := s.coder.Execute(ctx, input)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if ctx.Err() != nil || attempt == s.profile.MaxRetries {
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
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, lastErr
}

func duration(value string) time.Duration {
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 5 * time.Minute
	}
	return parsed
}
