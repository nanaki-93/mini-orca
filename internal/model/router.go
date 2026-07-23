package model

import (
	"context"
	"fmt"
	"sync"
)

// Phase represents a phase in the Mini-Orca workflow.
type Phase string

const (
	PhasePlanning    Phase = "planning"
	PhaseCoding      Phase = "coding"
	PhaseTesting     Phase = "testing"
	PhaseReview      Phase = "review"
	PhaseHumanReview Phase = "human_review"
)

// PhaseConfig holds the model configuration for a specific phase.
type PhaseConfig struct {
	Provider    string  `yaml:"provider"`
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature,omitempty"`
	MaxTokens   int     `yaml:"max_tokens,omitempty"`
	TopP        float64 `yaml:"top_p,omitempty"`
}

// Config holds the complete model configuration.
type Config struct {
	ActiveProvider string               `yaml:"active_provider"`
	Providers      map[string]ProviderConfig `yaml:"providers"`
	Phases         map[Phase]PhaseConfig     `yaml:"phases"`
}

// ProviderConfig holds provider-specific connection settings.
type ProviderConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key,omitempty"`
}

// Router manages model routing based on the current phase.
type Router struct {
	config      Config
	providers   map[string]Provider
	modelConfig map[string]ModelConfig
	mu          sync.RWMutex
}

// NewRouter creates a new Router with the given configuration.
func NewRouter(config Config) *Router {
	r := &Router{
		config:      config,
		providers:   make(map[string]Provider),
		modelConfig: make(map[string]ModelConfig),
	}

	// Build model config map from phase configs
	for phase, pc := range config.Phases {
		providerCfg, ok := config.Providers[pc.Provider]
		if !ok {
			providerCfg = ProviderConfig{BaseURL: ""}
		}
		r.modelConfig[string(phase)] = ModelConfig{
			Provider:    pc.Provider,
			Model:       pc.Model,
			BaseURL:     providerCfg.BaseURL,
			APIKey:      providerCfg.APIKey,
			Temperature: pc.Temperature,
			MaxTokens:   pc.MaxTokens,
			TopP:        pc.TopP,
		}
	}

	return r
}

// RegisterProvider registers a provider implementation by name.
func (r *Router) RegisterProvider(name string, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
}

// GetProviderForPhase returns the provider for the given phase.
func (r *Router) GetProviderForPhase(phase Phase) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg, ok := r.modelConfig[string(phase)]
	if !ok {
		return nil, fmt.Errorf("no model configuration for phase: %s", phase)
	}

	provider, ok := r.providers[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("provider not registered: %s", cfg.Provider)
	}

	return provider, nil
}

// GetModelConfigForPhase returns the model configuration for the given phase.
func (r *Router) GetModelConfigForPhase(phase Phase) (ModelConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg, ok := r.modelConfig[string(phase)]
	if !ok {
		return ModelConfig{}, fmt.Errorf("no model configuration for phase: %s", phase)
	}

	return cfg, nil
}

// Chat sends a chat request through the router, automatically selecting
// the appropriate provider and model based on the phase.
func (r *Router) Chat(ctx context.Context, phase Phase, messages []Message, options ...ChatOption) (*ChatResponse, error) {
	provider, err := r.GetProviderForPhase(phase)
	if err != nil {
		return nil, err
	}

	modelCfg, err := r.GetModelConfigForPhase(phase)
	if err != nil {
		return nil, err
	}

	req := ChatRequest{
		Model:    modelCfg.Model,
		Messages: messages,
	}

	// Apply options
	for _, opt := range options {
		opt(&req)
	}

	// Apply defaults from phase config if not overridden
	if req.Temperature == nil {
		t := modelCfg.Temperature
		req.Temperature = &t
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = modelCfg.MaxTokens
	}
	if req.TopP == nil {
		t := modelCfg.TopP
		req.TopP = &t
	}

	return provider.Chat(ctx, req)
}

// ChatOption is a functional option for configuring chat requests.
type ChatOption func(*ChatRequest)

// WithTemperature sets the temperature for a chat request.
func WithTemperature(t float64) ChatOption {
	return func(r *ChatRequest) {
		r.Temperature = &t
	}
}

// WithMaxTokens sets the max tokens for a chat request.
func WithMaxTokens(m int) ChatOption {
	return func(r *ChatRequest) {
		r.MaxTokens = m
	}
}

// WithTopP sets the top_p for a chat request.
func WithTopP(t float64) ChatOption {
	return func(r *ChatRequest) {
		r.TopP = &t
	}
}

// ListAvailableModels returns the list of available models for a phase.
func (r *Router) ListAvailableModels(ctx context.Context, phase Phase) ([]string, error) {
	provider, err := r.GetProviderForPhase(phase)
	if err != nil {
		return nil, err
	}
	return provider.ListModels(ctx)
}

// GetConfig returns a copy of the router's configuration.
func (r *Router) GetConfig() Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}
