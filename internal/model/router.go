package model

import (
	"context"
	"fmt"
)

// Router routes requests to the appropriate LLM provider based on phase or explicit config.
// It manages provider registration, model selection, and phase-based defaults.
type Router struct {
	providers map[string]Provider    // providerName -> Provider instance
	defaults  map[string]ModelConfig // key (e.g., "phase:planning") -> default config
	active    string                 // Currently active provider name
}

// NewRouter creates a new Router instance with initialized maps.
func NewRouter() *Router {
	return &Router{
		providers: make(map[string]Provider),
		defaults:  make(map[string]ModelConfig),
	}
}

// RegisterProvider adds a provider to the router.
// If a provider with the same name already exists, it will be overwritten.
func (r *Router) RegisterProvider(p Provider) {
	r.providers[p.Name()] = p
	if r.active == "" {
		r.active = p.Name()
	}
}

// SetDefaultConfig sets the default model configuration for a given key.
// Keys can be phase names (e.g., "planning", "coding") or custom identifiers.
func (r *Router) SetDefaultConfig(key string, config ModelConfig) {
	r.defaults[key] = config
}

// GetDefaultConfig returns the default model configuration for a given key.
// Returns an empty ModelConfig if no default is set for the key.
func (r *Router) GetDefaultConfig(key string) ModelConfig {
	if config, ok := r.defaults[key]; ok {
		return config
	}
	return ModelConfig{}
}

// GetProvider returns the provider instance for the given name.
// Returns an error if the provider is not registered.
func (r *Router) GetProvider(name string) (Provider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return provider, nil
}

// GetActiveProvider returns the currently active provider.
func (r *Router) GetActiveProvider() (Provider, error) {
	return r.GetProvider(r.active)
}

// ListProviders returns the names of all registered providers.
func (r *Router) ListProviders() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// SetActiveProvider sets the active provider by name.
func (r *Router) SetActiveProvider(name string) error {
	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("cannot set active provider: %q not registered", name)
	}
	r.active = name
	return nil
}

// RouteChat routes a chat request to the appropriate provider based on the phase.
// It uses the phase-based default config if available, otherwise falls back to the active provider.
func (r *Router) RouteChat(ctx context.Context, phase string, messages []ChatMessage) (*ChatResponse, error) {
	// Get phase-specific config
	config := r.GetDefaultConfig(phase)

	// If no phase config, use active provider
	if config.Provider == "" {
		provider, err := r.GetActiveProvider()
		if err != nil {
			return nil, err
		}
		config.Provider = provider.Name()
	}

	// Get the provider
	provider, err := r.GetProvider(config.Provider)
	if err != nil {
		return nil, err
	}

	// Build the request
	req := ChatRequest{
		Model:       config.ModelID,
		Messages:    messages,
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
	}

	// Send the request
	return provider.Chat(ctx, req)
}

// GetPhaseConfig returns the PhaseConfig for a given phase.
// Returns an error if the phase is not configured.
func (r *Router) GetPhaseConfig(phase string) (*PhaseConfig, error) {
	config := r.GetDefaultConfig(phase)
	if config.Provider == "" && config.ModelID == "" {
		return nil, fmt.Errorf("phase %q not configured", phase)
	}
	return &PhaseConfig{
		ModelConfig: config,
	}, nil
}

// Chat routes a chat request to the appropriate provider based on the phase.
// It uses the phase-based default config if available, otherwise falls back to the active provider.
func (r *Router) Chat(phase string, messages []ChatMessage) (*ChatResponse, error) {
	// Get phase-specific config
	config := r.GetDefaultConfig(phase)

	// If no phase config, use active provider
	if config.Provider == "" {
		provider, err := r.GetActiveProvider()
		if err != nil {
			return nil, err
		}
		config.Provider = provider.Name()
	}

	// Get the provider
	provider, err := r.GetProvider(config.Provider)
	if err != nil {
		return nil, err
	}

	// Build the request
	req := ChatRequest{
		Model:       config.ModelID,
		Messages:    messages,
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
	}

	// Send the request
	return provider.Chat(context.Background(), req)
}

// RouteListModels routes a model listing request to the active provider.
func (r *Router) RouteListModels(ctx context.Context) ([]Model, error) {
	provider, err := r.GetActiveProvider()
	if err != nil {
		return nil, err
	}
	return provider.ListModels(ctx)
}
