package config

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"regexp"
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
	Provider         ModelProvider
	CLIPath          string
	APIBaseURL       string
	APIKey           string
	AccessToken      string `json:",omitempty"`
	VertexProject    string `json:",omitempty"`
	VertexLocation   string `json:",omitempty"`
	Model            string
	ReasoningEffort  string
	Temperature      float32
	TopP             *float32
	TopK             *int
	MinP             *float32
	PresencePenalty  *float32
	RepeatPenalty    *float32
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
	if configured.Provider == AgyProvider || configured.Provider == PiProvider {
		return resolveCLIProfile(scope, configured)
	}
	if configured.Provider == GeminiProvider || configured.Provider == VertexProvider {
		return resolveGoogleProfile(scope, configured)
	}
	if configured.Provider != "" && configured.Provider != OpenAIProvider {
		return ModelProfile{}, fmt.Errorf("model_scopes.%s.provider must be openai, gemini, vertex, agy, or pi", scope)
	}
	if hasGoogleSettings(configured) {
		return ModelProfile{}, fmt.Errorf("model_scopes.%s: Google backend settings require a native Google provider", scope)
	}
	return resolveHTTPProfile(scope, configured)
}

func resolveHTTPProfile(scope ModelScope, configured ModelProfileConfig) (ModelProfile, error) {
	if configured.CLIPath != "" {
		return ModelProfile{}, fmt.Errorf("model_scopes.%s.cli_path requires an agy or pi provider", scope)
	}
	if strings.TrimSpace(configured.APIBaseURL) == "" {
		if profileConfigHasValues(configured) {
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
		Provider:         OpenAIProvider,
		APIBaseURL:       apiBase,
		APIKey:           configured.APIKey,
		Model:            strings.TrimSpace(configured.Model),
		ReasoningEffort:  strings.TrimSpace(configured.ReasoningEffort),
		Temperature:      0.7,
		MaxTokens:        8192,
		ContextMaxTokens: defaultContextBudget(scope),
	}
	if configured.Provider != "" {
		profile.Provider = configured.Provider
	}
	if configured.Temperature != nil {
		profile.Temperature = *configured.Temperature
	}
	profile.TopP = configured.TopP
	profile.TopK = configured.TopK
	profile.MinP = configured.MinP
	profile.PresencePenalty = configured.PresencePenalty
	profile.RepeatPenalty = configured.RepeatPenalty
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

func (p ModelProfile) IsGoogle() bool {
	return p.Provider == GeminiProvider || p.Provider == VertexProvider
}

func hasGoogleSettings(configured ModelProfileConfig) bool {
	return configured.AccessToken != "" || configured.VertexProject != "" || configured.VertexLocation != ""
}

var googleIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,255}$`)
var googleLocation = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

func resolveGoogleProfile(scope ModelScope, configured ModelProfileConfig) (ModelProfile, error) {
	if err := validateGoogleAuthentication(scope, configured); err != nil {
		return ModelProfile{}, err
	}
	if err := validateGoogleParameters(scope, configured); err != nil {
		return ModelProfile{}, err
	}
	if configured.APIBaseURL == "" {
		configured.APIBaseURL = googleBaseURL(configured)
	}
	profile, err := resolveHTTPProfile(scope, configured)
	if err != nil {
		return ModelProfile{}, err
	}
	endpoint, _ := url.Parse(profile.APIBaseURL) // resolveHTTPProfile validated it.
	address := net.ParseIP(endpoint.Hostname())
	if endpoint.Scheme != "https" && endpoint.Hostname() != "localhost" && (address == nil || !address.IsLoopback()) {
		return ModelProfile{}, fmt.Errorf("model_scopes.%s: native Google endpoints require HTTPS or loopback HTTP", scope)
	}
	profile.AccessToken, profile.VertexProject, profile.VertexLocation = configured.AccessToken, configured.VertexProject, configured.VertexLocation
	return profile, nil
}

func googleBaseURL(configured ModelProfileConfig) string {
	if configured.Provider == GeminiProvider {
		return "https://generativelanguage.googleapis.com"
	}
	if configured.VertexLocation == "" || configured.VertexLocation == "global" {
		return "https://aiplatform.googleapis.com"
	}
	return "https://" + configured.VertexLocation + "-aiplatform.googleapis.com"
}

func validateGoogleAuthentication(scope ModelScope, configured ModelProfileConfig) error {
	for _, credential := range []string{configured.APIKey, configured.AccessToken} {
		if len(credential) > 8192 || strings.ContainsAny(credential, "\r\n\x00") || strings.TrimSpace(credential) != credential {
			return fmt.Errorf("model_scopes.%s: invalid Google credential format", scope)
		}
	}
	if configured.Provider == GeminiProvider {
		if hasGoogleSettings(configured) || configured.APIKey == "" {
			return fmt.Errorf("model_scopes.%s: gemini requires an explicit api_key and no Vertex settings", scope)
		}
		return nil
	}
	if configured.APIKey != "" {
		if hasGoogleSettings(configured) {
			return fmt.Errorf("model_scopes.%s: Vertex Express api_key cannot be combined with access_token or project/location", scope)
		}
		return nil
	}
	if configured.AccessToken == "" || !googleIdentifier.MatchString(configured.VertexProject) || !googleLocation.MatchString(configured.VertexLocation) {
		return fmt.Errorf("model_scopes.%s: Vertex requires an explicit access_token, vertex_project, and vertex_location, or a Vertex Express api_key", scope)
	}
	return nil
}

func validateGoogleParameters(scope ModelScope, configured ModelProfileConfig) error {
	if !googleIdentifier.MatchString(configured.Model) {
		return fmt.Errorf("model_scopes.%s.model must be a Google model identifier without a resource path", scope)
	}
	if configured.MinP != nil || configured.RepeatPenalty != nil {
		return fmt.Errorf("model_scopes.%s: native Google providers do not support min_p or repeat_penalty", scope)
	}
	if configured.TopK != nil && *configured.TopK > math.MaxInt32 {
		return fmt.Errorf("model_scopes.%s.top_k exceeds the native provider limit", scope)
	}
	switch configured.ReasoningEffort {
	case "", "none", "minimal", "low", "medium", "high":
		return nil
	default:
		return fmt.Errorf("model_scopes.%s.reasoning_effort is unsupported by the native Google provider", scope)
	}
}

func profileConfigHasValues(configured ModelProfileConfig) bool {
	return configured.Provider != "" || configured.CLIPath != "" || configured.APIKey != "" ||
		strings.TrimSpace(configured.Model) != "" ||
		strings.TrimSpace(configured.ReasoningEffort) != "" ||
		configured.Temperature != nil ||
		configured.TopP != nil ||
		configured.TopK != nil ||
		configured.MinP != nil ||
		configured.PresencePenalty != nil ||
		configured.RepeatPenalty != nil ||
		configured.MaxTokens != nil ||
		configured.ContextMaxTokens != nil
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
	if err := validateOptionalFloat(scope, "temperature", &profile.Temperature, 0, 2, true); err != nil {
		return err
	}
	if err := validateOptionalFloat(scope, "top_p", profile.TopP, 0, 1, true); err != nil {
		return err
	}
	if profile.TopK != nil && *profile.TopK < 0 {
		return fmt.Errorf("model_scopes.%s.top_k must be at least 0", scope)
	}
	if err := validateOptionalFloat(scope, "min_p", profile.MinP, 0, 1, true); err != nil {
		return err
	}
	if err := validateOptionalFloat(scope, "presence_penalty", profile.PresencePenalty, -2, 2, true); err != nil {
		return err
	}
	if err := validateOptionalFloat(scope, "repeat_penalty", profile.RepeatPenalty, 0, 2, false); err != nil {
		return err
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

func validateOptionalFloat(scope ModelScope, field string, value *float32, minimum, maximum float32, includeMinimum bool) error {
	if value == nil {
		return nil
	}
	if math.IsNaN(float64(*value)) || math.IsInf(float64(*value), 0) || *value > maximum || (includeMinimum && *value < minimum) || (!includeMinimum && *value <= minimum) {
		comparison := "between"
		if !includeMinimum {
			comparison = "greater than"
		}
		if comparison == "greater than" {
			return fmt.Errorf("model_scopes.%s.%s must be greater than %v and at most %v", scope, field, minimum, maximum)
		}
		return fmt.Errorf("model_scopes.%s.%s must be between %v and %v", scope, field, minimum, maximum)
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
