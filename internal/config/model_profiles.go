package config

import (
	"fmt"
	"net/url"
	"strings"
)

// ModelScope identifies one of the fixed, prompt-bearing model operations.
type ModelScope string

const (
	AnalyzeModelScope  ModelScope = "analyze"
	BugModelScope      ModelScope = "bug"
	FunctionModelScope ModelScope = "function"

	// MaxModelOutputTokens and MaxModelContextTokens bound configuration before
	// prompt assembly or a provider request begins.
	MaxModelOutputTokens  = 65536
	MaxModelContextTokens = 120000
)

// ModelProfile is a complete immutable model configuration selected at startup.
type ModelProfile struct {
	Scope            ModelScope
	APIBaseURL       string
	APIKey           string
	Model            string
	Temperature      float32
	MaxTokens        int
	ContextMaxTokens int
}

// ModelProfiles contains every resolved fixed scope.
type ModelProfiles struct {
	Analyze  ModelProfile
	Bug      ModelProfile
	Function ModelProfile
}

// ForScope returns one fixed resolved profile.
func (p ModelProfiles) ForScope(scope ModelScope) (ModelProfile, bool) {
	switch scope {
	case AnalyzeModelScope:
		return p.Analyze, true
	case BugModelScope:
		return p.Bug, true
	case FunctionModelScope:
		return p.Function, true
	default:
		return ModelProfile{}, false
	}
}

// ResolveModelProfiles applies the scoped-model fallback contract once at the
// configuration boundary. It never modifies the source Config.
func ResolveModelProfiles(cfg *Config) (ModelProfiles, error) {
	if cfg == nil {
		return ModelProfiles{}, fmt.Errorf("model profile configuration is required")
	}

	legacyBase, err := legacyAPIBaseURL(cfg.LLM.BaseURL)
	if err != nil {
		return ModelProfiles{}, err
	}

	legacy := ModelProfile{
		APIBaseURL:  legacyBase,
		APIKey:      cfg.LLM.APIKey,
		Model:       cfg.LLM.Model,
		Temperature: legacyTemperature(cfg.LLM.Temperature),
		MaxTokens:   legacyMaxTokens(cfg.LLM.MaxTokens),
	}
	profiles := ModelProfiles{}
	if profiles.Analyze, err = resolveProfile(AnalyzeModelScope, cfg.ModelScopes.Analyze, legacy, defaultContextBudget(AnalyzeModelScope)); err != nil {
		return ModelProfiles{}, err
	}
	if profiles.Bug, err = resolveProfile(BugModelScope, cfg.ModelScopes.Bug, legacy, defaultContextBudget(BugModelScope)); err != nil {
		return ModelProfiles{}, err
	}
	functionLegacy := legacy
	if cfg.Agents.Coder.Model != "" {
		functionLegacy.Model = cfg.Agents.Coder.Model
	}
	if profiles.Function, err = resolveProfile(FunctionModelScope, cfg.ModelScopes.Function, functionLegacy, defaultContextBudget(FunctionModelScope)); err != nil {
		return ModelProfiles{}, err
	}
	return profiles, nil
}

func legacyTemperature(value float32) float32 {
	if value == 0 {
		return 0.7
	}
	return value
}

func legacyMaxTokens(value int) int {
	if value == 0 {
		return 8192
	}
	return value
}

func resolveProfile(scope ModelScope, configured ModelProfileConfig, fallback ModelProfile, defaultContext int) (ModelProfile, error) {
	if configured.APIBaseURL == "" {
		if configured.Model != "" || configured.APIKey != "" || configured.Temperature != nil || configured.MaxTokens != nil || configured.ContextMaxTokens != nil {
			return ModelProfile{}, fmt.Errorf("model_scopes.%s.api_base_url is required when configuring a scope", scope)
		}
		fallback.Scope = scope
		fallback.ContextMaxTokens = defaultContext
		return fallback, nil
	}

	apiBase, err := validateAPIBaseURL(scope, configured.APIBaseURL)
	if err != nil {
		return ModelProfile{}, err
	}
	if strings.TrimSpace(configured.Model) == "" {
		return ModelProfile{}, fmt.Errorf("model_scopes.%s.model is required when api_base_url is configured", scope)
	}

	profile := ModelProfile{
		Scope:            scope,
		APIBaseURL:       apiBase,
		APIKey:           configured.APIKey,
		Model:            configured.Model,
		Temperature:      fallback.Temperature,
		MaxTokens:        fallback.MaxTokens,
		ContextMaxTokens: defaultContext,
	}
	if configured.Temperature != nil {
		profile.Temperature = *configured.Temperature
	}
	if configured.MaxTokens != nil {
		profile.MaxTokens = *configured.MaxTokens
	}
	if configured.ContextMaxTokens != nil {
		profile.ContextMaxTokens = *configured.ContextMaxTokens
	}
	if err := validateProfileValues(scope, profile); err != nil {
		return ModelProfile{}, err
	}
	return profile, nil
}

func defaultContextBudget(scope ModelScope) int {
	switch scope {
	case AnalyzeModelScope:
		return 120000
	case BugModelScope:
		return 32000
	case FunctionModelScope:
		return 4000
	default:
		return 0
	}
}

func legacyAPIBaseURL(rawURL string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if trimmed == "" {
		return "", fmt.Errorf("llm.base_url is required")
	}
	return trimmed + "/v1", nil
}

func validateAPIBaseURL(scope ModelScope, rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("model_scopes.%s.api_base_url must be an absolute HTTP(S) URL", scope)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("model_scopes.%s.api_base_url must use HTTP(S)", scope)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("model_scopes.%s.api_base_url must not include user information, query, or fragment", scope)
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func validateProfileValues(scope ModelScope, profile ModelProfile) error {
	if profile.Temperature < 0 || profile.Temperature > 2 {
		return fmt.Errorf("model_scopes.%s.temperature must be between 0 and 2", scope)
	}
	if profile.MaxTokens <= 0 || profile.MaxTokens > MaxModelOutputTokens {
		return fmt.Errorf("model_scopes.%s.max_tokens must be between 1 and %d", scope, MaxModelOutputTokens)
	}
	if profile.ContextMaxTokens <= 0 || profile.ContextMaxTokens > MaxModelContextTokens {
		return fmt.Errorf("model_scopes.%s.context_max_tokens must be between 1 and %d", scope, MaxModelContextTokens)
	}
	return nil
}
