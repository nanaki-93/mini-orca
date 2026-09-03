package config

import (
	"strings"
	"testing"
)

func TestResolveModelProfilesLegacyFallback(t *testing.T) {
	cfg := &Config{
		LLM:    LLMConfig{BaseURL: "http://localhost:1234", APIKey: "legacy-key", Model: "legacy-model", Temperature: 0.3, MaxTokens: 1024},
		Agents: AgentsConfig{Coder: AgentConfig{Model: "coder-model"}},
	}

	profiles, err := ResolveModelProfiles(cfg)
	if err != nil {
		t.Fatalf("ResolveModelProfiles() error = %v", err)
	}
	for _, profile := range []ModelProfile{profiles.Analyze, profiles.Bug} {
		if profile.APIBaseURL != "http://localhost:1234/v1" || profile.Model != "legacy-model" || profile.APIKey != "legacy-key" {
			t.Fatalf("legacy profile = %+v", profile)
		}
	}
	if profiles.Function.Model != "coder-model" {
		t.Fatalf("function model = %q, want coder override", profiles.Function.Model)
	}
	if profiles.Analyze.ContextMaxTokens != 120000 || profiles.Bug.ContextMaxTokens != 32000 || profiles.Function.ContextMaxTokens != 4000 {
		t.Fatalf("unexpected context budgets: %+v", profiles)
	}
}

func TestResolveModelProfilesIndependentExplicitProfiles(t *testing.T) {
	zero := float32(0)
	maxTokens := 2048
	contextTokens := 6000
	cfg := &Config{
		LLM: LLMConfig{BaseURL: "http://localhost:1234", Model: "legacy", Temperature: 0.7, MaxTokens: 8192},
		ModelScopes: ModelScopesConfig{
			Analyze:  ModelProfileConfig{APIBaseURL: "https://analysis.example/v1", APIKey: "remote-key", Model: "analysis-model", Temperature: &zero, MaxTokens: &maxTokens, ContextMaxTokens: &contextTokens},
			Bug:      ModelProfileConfig{APIBaseURL: "https://bugs.example/v1beta/openai/", Model: "bug-model"},
			Function: ModelProfileConfig{APIBaseURL: "http://localhost:11434/v1", Model: "function-model"},
		},
	}

	profiles, err := ResolveModelProfiles(cfg)
	if err != nil {
		t.Fatalf("ResolveModelProfiles() error = %v", err)
	}
	if profiles.Analyze.Temperature != 0 || profiles.Analyze.MaxTokens != 2048 || profiles.Analyze.ContextMaxTokens != 6000 {
		t.Fatalf("analyze optional values = %+v", profiles.Analyze)
	}
	if profiles.Bug.APIBaseURL != "https://bugs.example/v1beta/openai" || profiles.Bug.APIKey != "" || profiles.Bug.Model != "bug-model" {
		t.Fatalf("bug profile = %+v", profiles.Bug)
	}
	if profiles.Function.APIBaseURL != "http://localhost:11434/v1" || profiles.Function.Model != "function-model" {
		t.Fatalf("function profile = %+v", profiles.Function)
	}
}

func TestResolveModelProfilesRejectsUnsafeOrPartialProfiles(t *testing.T) {
	secret := "not-for-errors"
	tests := []struct {
		name    string
		profile ModelProfileConfig
	}{
		{name: "missing API base", profile: ModelProfileConfig{Model: "model"}},
		{name: "missing model", profile: ModelProfileConfig{APIBaseURL: "https://provider.example/v1", APIKey: secret}},
		{name: "user information", profile: ModelProfileConfig{APIBaseURL: "https://user:password@provider.example/v1", Model: "model", APIKey: secret}},
		{name: "query", profile: ModelProfileConfig{APIBaseURL: "https://provider.example/v1?secret=value", Model: "model", APIKey: secret}},
		{name: "invalid scheme", profile: ModelProfileConfig{APIBaseURL: "file:///tmp/provider", Model: "model", APIKey: secret}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ResolveModelProfiles(&Config{
				LLM:         LLMConfig{BaseURL: "http://localhost:1234", Model: "legacy"},
				ModelScopes: ModelScopesConfig{Analyze: test.profile},
			})
			if err == nil {
				t.Fatal("ResolveModelProfiles() error = nil")
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "password") {
				t.Fatalf("error leaked secret: %v", err)
			}
		})
	}
}

func TestResolveModelProfilesRejectsInvalidNumericValues(t *testing.T) {
	zero := 0
	overLimit := MaxModelOutputTokens + 1
	negativeTemperature := float32(-0.1)
	tests := []struct {
		name    string
		profile ModelProfileConfig
	}{
		{name: "zero max tokens", profile: ModelProfileConfig{APIBaseURL: "http://localhost:11434/v1", Model: "model", MaxTokens: &zero}},
		{name: "output limit", profile: ModelProfileConfig{APIBaseURL: "http://localhost:11434/v1", Model: "model", MaxTokens: &overLimit}},
		{name: "temperature", profile: ModelProfileConfig{APIBaseURL: "http://localhost:11434/v1", Model: "model", Temperature: &negativeTemperature}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ResolveModelProfiles(&Config{
				LLM:         LLMConfig{BaseURL: "http://localhost:1234", Model: "legacy"},
				ModelScopes: ModelScopesConfig{Function: test.profile},
			})
			if err == nil {
				t.Fatal("ResolveModelProfiles() error = nil")
			}
		})
	}
}

func TestResolveModelProfilesForScope(t *testing.T) {
	profiles, err := ResolveModelProfiles(Default())
	if err != nil {
		t.Fatalf("ResolveModelProfiles() error = %v", err)
	}
	if _, ok := profiles.ForScope("unknown"); ok {
		t.Fatal("unknown scope unexpectedly resolved")
	}
}
