package config

import (
	"strings"
	"testing"
)

func TestLoadCLIProfilesAlongsideHTTP(t *testing.T) {
	cfg, err := LoadFromYAML(writeConfig(t, `
model_scopes:
  analyze:
    provider: agy
    model: gemini-3.8-flash-high
    reasoning_effort: high
  bug:
    provider: openai
    api_base_url: http://localhost:1234/v1
    model: local-model
  function:
    provider: pi
    cli_path: /opt/agent tools/pi
    model: test-provider/test-model
    reasoning_effort: none
    context_max_tokens: 8000
`))
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := ResolveModelProfiles(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !profiles.Analyze.IsCLI() || profiles.Analyze.CLIPath != "agy" || profiles.Analyze.APIBaseURL != "" || profiles.Analyze.ContextMaxTokens != 120000 || profiles.Analyze.MaxTokens != 0 {
		t.Fatalf("agy profile = %+v", profiles.Analyze)
	}
	if profiles.Bug.IsCLI() || profiles.Bug.Provider != OpenAIProvider || profiles.Function.CLIPath != "/opt/agent tools/pi" || profiles.Function.ContextMaxTokens != 8000 || profiles.Function.ReasoningEffort != "none" {
		t.Fatalf("mixed profiles = %+v", profiles)
	}
}

func TestCLIProfilesRejectAmbiguousOrUnsupportedSettings(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*ModelProfileConfig)
		want string
	}{
		{"unknown provider", func(p *ModelProfileConfig) { p.Provider = "shell" }, "provider must be"},
		{"missing model", func(p *ModelProfileConfig) { p.Model = " " }, "model is required"},
		{"model option", func(p *ModelProfileConfig) { p.Model = "--unsafe" }, "model identifier"},
		{"agy display name", func(p *ModelProfileConfig) { p.Model = "Gemini 3.8 Flash" }, "slug from agy models"},
		{"agy model whitespace", func(p *ModelProfileConfig) { p.Model = "gemini-3.8-flash\thigh" }, "slug from agy models"},
		{"API destination", func(p *ModelProfileConfig) { p.APIBaseURL = "https://example.test" }, "omit api_base_url"},
		{"API credential", func(p *ModelProfileConfig) { p.APIKey = "private-marker" }, "own authentication"},
		{"output tokens", func(p *ModelProfileConfig) { n := 100; p.MaxTokens = &n }, "output-token controls"},
		{"temperature", func(p *ModelProfileConfig) { n := float32(0); p.Temperature = &n }, "sampling"},
		{"sampling", func(p *ModelProfileConfig) { n := 1; p.TopK = &n }, "sampling"},
		{"relative executable", func(p *ModelProfileConfig) { p.CLIPath = "./agent" }, "absolute executable"},
		{"shell command", func(p *ModelProfileConfig) { p.CLIPath = "agy --unsafe" }, "command name"},
		{"NUL executable", func(p *ModelProfileConfig) { p.CLIPath = "agy\x00" }, "command name"},
		{"agy effort", func(p *ModelProfileConfig) { p.ReasoningEffort = "minimal" }, "unsupported by agy"},
		{"context budget", func(p *ModelProfileConfig) { n := 0; p.ContextMaxTokens = &n }, "context_max_tokens"},
		{"HTTP executable", func(p *ModelProfileConfig) { p.Provider = OpenAIProvider; p.CLIPath = "agy" }, "requires an agy or pi"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := Default()
			cfg.ModelScopes.Function = ModelProfileConfig{Provider: AgyProvider, Model: "test-model"}
			test.edit(&cfg.ModelScopes.Function)
			_, err := ResolveModelProfiles(cfg)
			if err == nil || !strings.Contains(err.Error(), "model_scopes.function") || !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), "private-marker") {
				t.Fatalf("configuration error = %v", err)
			}
		})
	}
}
