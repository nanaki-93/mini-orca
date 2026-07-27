package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config is the top-level configuration struct.
type Config struct {
	Models ModelsConfig `json:"models"`
	Agents AgentsConfig `json:"agents"`
	Skills SkillsConfig `json:"skills"`
}

// ModelsConfig holds model-related configuration.
type ModelsConfig struct {
	ActiveProvider string                      `json:"active_provider"`
	Providers      map[string]ProviderConfig   `json:"providers"`
	Phases         map[string]PhaseModelConfig `json:"phases"`
}

// ProviderConfig holds provider-specific configuration.
type ProviderConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
}

// PhaseModelConfig holds model configuration for a specific phase.
type PhaseModelConfig struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Temperature float32 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// AgentsConfig holds agent-related configuration.
type AgentsConfig struct {
	Planner  AgentConfig `json:"planner"`
	Coder    AgentConfig `json:"coder"`
	Tester   AgentConfig `json:"tester"`
	Reviewer AgentConfig `json:"reviewer"`
}

// AgentConfig holds configuration for a single agent.
type AgentConfig struct {
	Skills []string `json:"skills"`
	Model  string   `json:"model,omitempty"` // optional override
}

// SkillsConfig holds skill-related configuration.
type SkillsConfig struct {
	Knowledge map[string]string `json:"knowledge"`
	Tools     map[string]string `json:"tools"`
}

// Load reads configuration from a JSON file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Initialize maps to avoid nil map issues
	if cfg.Models.Providers == nil {
		cfg.Models.Providers = make(map[string]ProviderConfig)
	}
	if cfg.Models.Phases == nil {
		cfg.Models.Phases = make(map[string]PhaseModelConfig)
	}
	if cfg.Skills.Knowledge == nil {
		cfg.Skills.Knowledge = make(map[string]string)
	}
	if cfg.Skills.Tools == nil {
		cfg.Skills.Tools = make(map[string]string)
	}

	return &cfg, nil
}

// Save writes configuration to a JSON file.
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.Models.ActiveProvider == "" {
		return fmt.Errorf("active_provider is required")
	}

	if _, ok := c.Models.Providers[c.Models.ActiveProvider]; !ok {
		return fmt.Errorf("active_provider %q not found in providers", c.Models.ActiveProvider)
	}

	return nil
}
