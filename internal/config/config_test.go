package config

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromYAMLAcceptsThreeCompleteScopedProfiles(t *testing.T) {
	path := writeConfig(t, `
model_scopes:
  analyze:
    api_base_url: "https://analysis.example/v1"
    api_key: "analysis-secret"
    model: "analysis-model"
    reasoning_effort: high
    temperature: 0
    top_p: 0
    top_k: 0
    min_p: 0
    presence_penalty: 0
    repeat_penalty: 1
    max_tokens: 16000
    context_max_tokens: 120000
  bug:
    api_base_url: "http://localhost:11434/v1"
    api_key: ""
    model: "bug-model"
  function:
    api_base_url: "http://localhost:1234/v1"
    model: "function-model"
`)
	cfg, err := LoadFromYAML(path)
	if err != nil {
		t.Fatalf("LoadFromYAML() error = %v", err)
	}
	profiles, err := ResolveModelProfiles(cfg)
	if err != nil {
		t.Fatalf("ResolveModelProfiles() error = %v", err)
	}
	if profiles.Analyze.Temperature != 0 || profiles.Analyze.MaxTokens != 16000 || profiles.Bug.APIKey != "" {
		t.Fatalf("resolved profiles = %+v", profiles)
	}
	if profiles.Analyze.TopP == nil || *profiles.Analyze.TopP != 0 || profiles.Analyze.TopK == nil || *profiles.Analyze.TopK != 0 || profiles.Analyze.MinP == nil || *profiles.Analyze.MinP != 0 || profiles.Analyze.PresencePenalty == nil || *profiles.Analyze.PresencePenalty != 0 || profiles.Analyze.RepeatPenalty == nil || *profiles.Analyze.RepeatPenalty != 1 {
		t.Fatalf("explicit sampling values were not preserved: %+v", profiles.Analyze)
	}
	if profiles.Bug.TopP != nil || profiles.Bug.TopK != nil || profiles.Bug.MinP != nil || profiles.Bug.PresencePenalty != nil || profiles.Bug.RepeatPenalty != nil {
		t.Fatalf("omitted sampling values became defaults: %+v", profiles.Bug)
	}
	if profiles.Bug.ContextMaxTokens != 32000 || profiles.Function.ContextMaxTokens != 4000 {
		t.Fatalf("default context budgets = %+v", profiles)
	}
	if cfg.Retry.MaxRetries != 3 || cfg.Timeouts.GenerationSeconds != 300 || cfg.Logging.Level != "info" {
		t.Fatalf("ordinary defaults were not applied: %+v", cfg)
	}
}

func TestLoadFromYAMLRejectsRetiredAndUnknownFieldsWithFullPaths(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{name: "retired flat LLM", body: "llm:\n  base_url: http://localhost:1234\n", want: `unsupported configuration field "llm"`},
		{name: "retired agents", body: "agents:\n  coder: {}\n", want: `unsupported configuration field "agents"`},
		{name: "misspelled profile field", body: "model_scopes:\n  analyze:\n    api_base_urll: http://localhost:1234/v1\n", want: `unsupported configuration field "model_scopes.analyze.api_base_urll"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadFromYAML(writeConfig(t, test.body))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadFromYAML() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLoadFromYAMLRejectsMissingAndPartialScopes(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{name: "missing scopes", body: "model_scopes: {}\n", want: "model_scopes.analyze is required"},
		{name: "profile without API base", body: "model_scopes:\n  analyze:\n    model: model\n", want: "model_scopes.analyze.api_base_url is required"},
		{name: "sampler without API base", body: "model_scopes:\n  analyze:\n    top_p: 0\n", want: "model_scopes.analyze.api_base_url is required"},
		{name: "partial profile", body: "model_scopes:\n  analyze:\n    api_base_url: http://localhost:1234/v1\n", want: "model_scopes.analyze.model is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadFromYAML(writeConfig(t, test.body))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadFromYAML() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestResolveModelProfilesRejectsUnsafeAndInvalidValuesWithoutSecret(t *testing.T) {
	temperature := float32(3)
	negativeTopK := -1
	nan := float32(math.NaN())
	infinity := float32(math.Inf(1))
	tooHigh := float32(3)
	zero := float32(0)
	for _, test := range []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{name: "user information", change: func(cfg *Config) { cfg.ModelScopes.Analyze.APIBaseURL = "https://user:password@example.test/v1" }, want: "must not include user information"},
		{name: "query", change: func(cfg *Config) { cfg.ModelScopes.Bug.APIBaseURL = "https://example.test/v1?token=secret" }, want: "must not include user information"},
		{name: "temperature", change: func(cfg *Config) { cfg.ModelScopes.Function.Temperature = &temperature }, want: "temperature must be between 0 and 2"},
		{name: "top p range", change: func(cfg *Config) { cfg.ModelScopes.Function.TopP = &tooHigh }, want: "top_p must be between 0 and 1"},
		{name: "minimum p nonfinite", change: func(cfg *Config) { cfg.ModelScopes.Function.MinP = &nan }, want: "min_p must be between 0 and 1"},
		{name: "presence penalty nonfinite", change: func(cfg *Config) { cfg.ModelScopes.Function.PresencePenalty = &infinity }, want: "presence_penalty must be between -2 and 2"},
		{name: "top k integer", change: func(cfg *Config) { cfg.ModelScopes.Function.TopK = &negativeTopK }, want: "top_k must be at least 0"},
		{name: "repeat penalty zero", change: func(cfg *Config) { cfg.ModelScopes.Function.RepeatPenalty = &zero }, want: "repeat_penalty must be greater than 0 and at most 2"},
		{name: "model", change: func(cfg *Config) { cfg.ModelScopes.Bug.Model = "" }, want: "model_scopes.bug.model is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := Default()
			cfg.ModelScopes.Analyze.APIKey = "do-not-return-this-secret"
			test.change(cfg)
			_, err := ResolveModelProfiles(cfg)
			if err == nil || !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), "do-not-return-this-secret") {
				t.Fatalf("ResolveModelProfiles() error = %v", err)
			}
		})
	}
}

func TestDefaultIsACompleteLocalScopedConfiguration(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Default().Validate() error = %v", err)
	}
	if cfg.ModelScopes.Function.APIBaseURL != DefaultProviderAPIBaseURL {
		t.Fatalf("function API base = %q", cfg.ModelScopes.Function.APIBaseURL)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
