package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type ModelProvider string

const (
	OpenAIProvider ModelProvider = "openai"
	AgyProvider    ModelProvider = "agy"
	PiProvider     ModelProvider = "pi"
	// ClaudeProvider runs Claude Code (the claude CLI) on its subscription login.
	ClaudeProvider ModelProvider = "claude"
	GeminiProvider ModelProvider = "gemini"
	VertexProvider ModelProvider = "vertex"
)

func (p ModelProfile) IsCLI() bool {
	return p.Provider == AgyProvider || p.Provider == PiProvider || p.Provider == ClaudeProvider
}

func resolveCLIProfile(scope ModelScope, configured ModelProfileConfig) (ModelProfile, error) {
	if err := validateCLISettings(scope, configured); err != nil {
		return ModelProfile{}, err
	}
	model := strings.TrimSpace(configured.Model)
	if err := validateCLIModel(scope, configured.Provider, model); err != nil {
		return ModelProfile{}, err
	}
	command := strings.TrimSpace(configured.CLIPath)
	if command == "" {
		command = string(configured.Provider)
	}
	if strings.ContainsAny(command, "\r\n\x00") || (!filepath.IsAbs(command) && (filepath.Base(command) != command || strings.ContainsAny(command, " \t"))) {
		return ModelProfile{}, fmt.Errorf("model_scopes.%s.cli_path must be an absolute executable path or a command name on PATH", scope)
	}
	effort := strings.TrimSpace(configured.ReasoningEffort)
	if !validCLIEffort(configured.Provider, effort) {
		return ModelProfile{}, fmt.Errorf("model_scopes.%s.reasoning_effort is unsupported by %s", scope, configured.Provider)
	}
	budget := defaultContextBudget(scope)
	if configured.ContextMaxTokens != nil {
		budget = *configured.ContextMaxTokens
	}
	if budget <= 0 || budget > MaxModelContextTokens {
		return ModelProfile{}, fmt.Errorf("model_scopes.%s.context_max_tokens must be between 1 and %d", scope, MaxModelContextTokens)
	}
	return ModelProfile{Scope: scope, Provider: configured.Provider, CLIPath: command, Model: model, ReasoningEffort: effort, ContextMaxTokens: budget}, nil
}

func validateCLIModel(scope ModelScope, provider ModelProvider, model string) error {
	if model == "" {
		return missingFieldError(scope, "model")
	}
	if strings.HasPrefix(model, "-") || strings.ContainsAny(model, "\r\n\x00") {
		return fmt.Errorf("model_scopes.%s.model must be a CLI model identifier", scope)
	}
	if provider == AgyProvider && !ValidAgyModelID(model) {
		return fmt.Errorf("model_scopes.%s.model must be a slug from agy models, not a display name", scope)
	}
	if provider == ClaudeProvider && !ValidClaudeModelID(model) {
		return fmt.Errorf("model_scopes.%s.model must be a Claude Code model alias or name, such as sonnet or claude-sonnet-5", scope)
	}
	return nil
}

func validateCLISettings(scope ModelScope, configured ModelProfileConfig) error {
	if hasGoogleSettings(configured) {
		return fmt.Errorf("model_scopes.%s: Google backend settings require a native Google provider", scope)
	}
	if configured.APIBaseURL != "" || configured.APIKey != "" {
		return fmt.Errorf("model_scopes.%s: CLI providers use their own authentication; omit api_base_url and api_key", scope)
	}
	if configured.Temperature != nil || configured.MaxTokens != nil || configured.TopP != nil || configured.TopK != nil || configured.MinP != nil || configured.PresencePenalty != nil || configured.RepeatPenalty != nil {
		return fmt.Errorf("model_scopes.%s: CLI providers do not expose sampling or output-token controls; omit temperature, max_tokens, top_p, top_k, min_p, presence_penalty, and repeat_penalty", scope)
	}
	return nil
}

var agyModelIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

func ValidAgyModelID(model string) bool {
	return len(model) <= 512 && agyModelIDPattern.MatchString(model)
}

var claudeModelIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*(\[[a-z0-9]+\])?$`)

// ValidClaudeModelID accepts Claude Code's aliases (sonnet, opus[1m]) and full
// model names.
func ValidClaudeModelID(model string) bool {
	return len(model) <= 128 && claudeModelIDPattern.MatchString(model)
}

func validCLIEffort(provider ModelProvider, effort string) bool {
	if provider == PiProvider {
		return validReasoningEffort(effort)
	}
	switch effort {
	case "", "low", "medium", "high", "xhigh", "max":
		return true
	default:
		return false
	}
}
