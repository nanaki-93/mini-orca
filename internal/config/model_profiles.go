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

	MaxModelOutputTokens  = 65536
	MaxModelContextTokens = 120000
)

// ModelProfile is a complete immutable model configuration selected at startup.
type ModelProfile struct {
	Scope            ModelScope
	APIBaseURL       string
	APIKey           string
	Model            string
	ReasoningEffort  string
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

// ResolveModelProfiles validates each declared scope independently. There is no
// flat profile, agent setting, or cross-scope fallback.
func ResolveModelProfiles(cfg *Config) (ModelProfiles, error) {
	if cfg == nil {
		return ModelProfiles{}, fmt.Errorf("model profile configuration is required")
	}
	profiles := ModelProfiles{}
	var err error
	if profiles.Analyze, err = resolveProfile(AnalyzeModelScope, cfg.ModelScopes.Analyze); err != nil {
		return ModelProfiles{}, err
	}
	if profiles.Bug, err = resolveProfile(BugModelScope, cfg.ModelScopes.Bug); err != nil {
		return ModelProfiles{}, err
	}
	if profiles.Function, err = resolveProfile(FunctionModelScope, cfg.ModelScopes.Function); err != nil {
		return ModelProfiles{}, err
	}
	return profiles, nil
}

func resolveProfile(scope ModelScope, configured ModelProfileConfig) (ModelProfile, error) {
	if strings.TrimSpace(configured.APIBaseURL) == "" {
		if configured.APIKey != "" || strings.TrimSpace(configured.Model) != "" || strings.TrimSpace(configured.ReasoningEffort) != "" || configured.Temperature != nil || configured.MaxTokens != nil || configured.ContextMaxTokens != nil {
			return ModelProfile{}, missingFieldError(scope, "api_base_url")
		}
		return ModelProfile{}, missingScopeError(scope)
	}
	apiBase, err := validateAPIBaseURL(scope, configured.APIBaseURL)
	if err != nil {
		return ModelProfile{}, err
	}
	if strings.TrimSpace(configured.Model) == "" {
		return ModelProfile{}, missingFieldError(scope, "model")
	}

	profile := ModelProfile{
		Scope:            scope,
		APIBaseURL:       apiBase,
		APIKey:           configured.APIKey,
		Model:            strings.TrimSpace(configured.Model),
		ReasoningEffort:  strings.TrimSpace(configured.ReasoningEffort),
		Temperature:      0.7,
		MaxTokens:        8192,
		ContextMaxTokens: defaultContextBudget(scope),
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

func validateAPIBaseURL(scope ModelScope, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
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
	if !validReasoningEffort(profile.ReasoningEffort) {
		return fmt.Errorf("model_scopes.%s.reasoning_effort must be one of none, minimal, low, medium, high, xhigh, or max", scope)
	}
	return nil
}

func validReasoningEffort(value string) bool {
	switch value {
	case "", "none", "minimal", "low", "medium", "high", "xhigh", "max":
		return true
	default:
		return false
	}
}
