package config

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeGoogleProfilesUseExplicitAuthentication(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "must-not-be-used")
	for name, configured := range map[string]ModelProfileConfig{
		"developer": {Provider: GeminiProvider, APIKey: "explicit-key", Model: "gemini-fixture"},
		"express":   {Provider: VertexProvider, APIKey: "express-key", Model: "gemini-fixture"},
		"vertex":    {Provider: VertexProvider, AccessToken: "explicit-token", VertexProject: "project-id", VertexLocation: "us-central1", Model: "gemini-fixture"},
	} {
		t.Run(name, func(t *testing.T) {
			profile, err := resolveProfile(AnalyzeModelScope, configured)
			if err != nil || !profile.IsGoogle() || profile.APIKey != configured.APIKey || profile.AccessToken != configured.AccessToken || profile.APIBaseURL != googleBaseURL(configured) || profile.ContextMaxTokens != 120000 {
				t.Fatalf("profile=%+v error=%v", profile, err)
			}
		})
	}
	for name, configured := range map[string]ModelProfileConfig{
		"ambient key":             {Provider: GeminiProvider, Model: "fixture"},
		"ambient token":           {Provider: VertexProvider, Model: "fixture", VertexProject: "project-id", VertexLocation: "global"},
		"mixed authentication":    {Provider: VertexProvider, Model: "fixture", APIKey: "key", AccessToken: "token"},
		"developer vertex fields": {Provider: GeminiProvider, Model: "fixture", APIKey: "key", VertexProject: "project-id"},
		"unsafe model path":       {Provider: GeminiProvider, Model: "../another", APIKey: "key"},
		"unsafe location":         {Provider: VertexProvider, Model: "fixture", AccessToken: "token", VertexProject: "project", VertexLocation: "../escape"},
		"header injection":        {Provider: GeminiProvider, Model: "fixture", APIKey: "key\r\nextra"},
		"unencrypted remote":      {Provider: GeminiProvider, Model: "fixture", APIKey: "key", APIBaseURL: "http://remote.example"},
		"unsupported effort":      {Provider: GeminiProvider, Model: "fixture", APIKey: "key", ReasoningEffort: "xhigh"},
		"compatible fields":       {APIBaseURL: "http://localhost:1234/v1", Model: "fixture", AccessToken: "token"},
		"CLI fields":              {Provider: PiProvider, Model: "provider/model", AccessToken: "token"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := resolveProfile(AnalyzeModelScope, configured); err == nil {
				t.Fatal("invalid Google settings accepted")
			}
		})
	}
}

func TestNativeGoogleYAMLRemainsStrict(t *testing.T) {
	text := `model_scopes:
  analyze: {provider: gemini, model: fixture, api_key: explicit-key}
  bug: {provider: vertex, model: fixture, access_token: explicit-token, vertex_project: project-id, vertex_location: global}
  code: {api_base_url: "http://localhost:1234/v1", model: local-model}
`
	if _, err := LoadFromYAML(writeConfig(t, text)); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFromYAML(writeConfig(t, strings.Replace(text, "vertex_location:", "vertex_locaton:", 1))); err == nil {
		t.Fatal("unknown native setting accepted")
	}
	for _, value := range []float32{float32(math.NaN()), float32(math.Inf(1))} {
		if _, err := resolveProfile(AnalyzeModelScope, ModelProfileConfig{Provider: GeminiProvider, Model: "fixture", APIKey: "explicit-key", Temperature: &value}); err == nil {
			t.Fatal("non-finite native temperature accepted")
		}
	}
}

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
  code:
    api_base_url: "http://localhost:1234/v1"
    model: "code-model"
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
	if profiles.Bug.ContextMaxTokens != 32000 || profiles.Code.ContextMaxTokens != 4000 {
		t.Fatalf("default context budgets = %+v", profiles)
	}
	if cfg.Retry.MaxRetries != 3 || cfg.Timeouts.GenerationSeconds != 300 || cfg.Timeouts.WorkflowModelSeconds != 900 || cfg.Logging.Level != "info" {
		t.Fatalf("ordinary defaults were not applied: %+v", cfg)
	}
}

func TestWorkflowModelTimeoutDefaultsAndOverride(t *testing.T) {
	profiles := "model_scopes:\n  analyze: {api_base_url: 'http://localhost:1234/v1', model: a}\n  bug: {api_base_url: 'http://localhost:1234/v1', model: b}\n  code: {api_base_url: 'http://localhost:1234/v1', model: f}\n"
	for _, test := range []struct {
		name, settings string
		want           int
	}{
		{"existing configuration", "generation_seconds: 300", 900},
		{"explicit default", "workflow_model_seconds: 0", 900},
		{"longer workflow", "workflow_model_seconds: 1800", 1800},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := LoadFromYAML(writeConfig(t, profiles+"timeouts:\n  "+test.settings+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Timeouts.WorkflowModelSeconds != test.want || cfg.Timeouts.GenerationSeconds != 300 {
				t.Fatalf("timeouts = %+v", cfg.Timeouts)
			}
		})
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
		{name: "temperature", change: func(cfg *Config) { cfg.ModelScopes.Code.Temperature = &temperature }, want: "temperature must be between 0 and 2"},
		{name: "top p range", change: func(cfg *Config) { cfg.ModelScopes.Code.TopP = &tooHigh }, want: "top_p must be between 0 and 1"},
		{name: "minimum p nonfinite", change: func(cfg *Config) { cfg.ModelScopes.Code.MinP = &nan }, want: "min_p must be between 0 and 1"},
		{name: "presence penalty nonfinite", change: func(cfg *Config) { cfg.ModelScopes.Code.PresencePenalty = &infinity }, want: "presence_penalty must be between -2 and 2"},
		{name: "top k integer", change: func(cfg *Config) { cfg.ModelScopes.Code.TopK = &negativeTopK }, want: "top_k must be at least 0"},
		{name: "repeat penalty zero", change: func(cfg *Config) { cfg.ModelScopes.Code.RepeatPenalty = &zero }, want: "repeat_penalty must be greater than 0 and at most 2"},
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
	if cfg.ModelScopes.Code.APIBaseURL != DefaultProviderAPIBaseURL {
		t.Fatalf("code API base = %q", cfg.ModelScopes.Code.APIBaseURL)
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
