package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentTriggersStrictConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	profiles := "model_scopes:\n  analyze: {api_base_url: 'http://localhost:1234/v1', model: a}\n  bug: {api_base_url: 'http://localhost:1234/v1', model: b}\n  code: {api_base_url: 'http://localhost:1234/v1', model: f}\n"
	for _, value := range []struct {
		yaml  string
		valid bool
	}{
		{"agent_runtime:\n  triggers:\n    poll_seconds: 5\n    max_runs: 2\n", true},
		{"agent_runtime:\n  triggers:\n    enabled: true\n", false},
		{"agent_runtime:\n  triggers:\n    grant_seconds: 86401\n", false},
	} {
		if err := os.WriteFile(path, []byte(profiles+value.yaml), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadFromYAML(path)
		if (err == nil) != value.valid {
			t.Fatalf("configuration validity=%t: %v", value.valid, err)
		}
		if err == nil {
			runtime, err := ResolveAgentRuntime(cfg.AgentRuntime)
			if err != nil || runtime.Triggers.PollSeconds != 5 || runtime.Triggers.MaxRuns != 2 {
				t.Fatal("trigger configuration not resolved", err)
			}
		}
	}
}

func TestAgentTriggersCeilingsAreBoundedAndDoNotChangeOrdinaryAdmission(t *testing.T) {
	runtime, err := ResolveAgentRuntime(AgentRuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(runtime)
	if strings.Contains(string(data), "triggers") || strings.Contains(string(data), "Triggers") {
		t.Fatal("ambient ceilings changed ordinary execution identity")
	}
	value := runtime.Triggers
	if value.MaxRuns != 4 || value.PollSeconds != 2 || value.GrantSeconds != 3600 {
		t.Fatal("unexpected default ceilings", value)
	}
	limits := value.RunLimits(AgentLimits{ModelCalls: 1, ToolCalls: 1, InputTokens: 2048, OutputTokens: 1024, ElapsedSeconds: 30, ConcurrentCalls: 1})
	if limits.ModelCalls != 1 || limits.InputTokens != 2048 || limits.OutputTokens != 1024 || limits.ElapsedSeconds != 30 || limits.Repairs != 0 {
		t.Fatal("trigger widened current execution limits", limits)
	}
	for _, change := range []func(*AgentTriggers){
		func(v *AgentTriggers) { v.PollSeconds = -1 }, func(v *AgentTriggers) { v.WindowSeconds = 86401 },
		func(v *AgentTriggers) { v.MaxRuns = 17 }, func(v *AgentTriggers) { v.MaxModelCalls = 1 },
		func(v *AgentTriggers) { v.MaxInputTokens = 65 << 20 }, func(v *AgentTriggers) { v.MaxOutputTokens = 17 << 20 },
		func(v *AgentTriggers) { v.RunSeconds = 301 }, func(v *AgentTriggers) { v.GrantSeconds = 86401 },
		func(v *AgentTriggers) { v.WindowSeconds, v.QuietSeconds = 60, 61 },
	} {
		invalid := value
		change(&invalid)
		if _, err := ResolveAgentTriggers(invalid); err == nil {
			t.Fatal("invalid trigger ceilings accepted", invalid)
		}
	}
}
