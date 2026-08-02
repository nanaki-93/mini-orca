package config

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration struct.
type Config struct {
	Models  ModelsConfig  `json:"models" yaml:"models"`
	Agents  AgentsConfig  `json:"agents" yaml:"agents"`
	Skills  SkillsConfig  `json:"skills" yaml:"skills"`
	Retry   RetryConfig   `json:"retry" yaml:"retry"`
	Logging LoggingConfig `json:"logging" yaml:"logging"`
}

// LoggingConfig holds configuration for the logger.
type LoggingConfig struct {
	Level         string   `json:"level" yaml:"level"`
	Format        string   `json:"format" yaml:"format"`
	Filename      string   `json:"filename" yaml:"filename"`
	SensitiveKeys []string `json:"sensitive_keys" yaml:"sensitive_keys"`
}

// ModelsConfig holds model-related configuration.
type ModelsConfig struct {
	ActiveProvider string                      `json:"active_provider" yaml:"active_provider"`
	Providers      map[string]ProviderConfig   `json:"providers" yaml:"providers"`
	Phases         map[string]PhaseModelConfig `json:"phases" yaml:"phases"`
}

// ProviderConfig holds provider-specific configuration.
type ProviderConfig struct {
	BaseURL string `json:"base_url" yaml:"base_url"`
	APIKey  string `json:"api_key,omitempty" yaml:"api_key,omitempty"`
}

// PhaseModelConfig holds model configuration for a specific phase.
type PhaseModelConfig struct {
	Provider    string  `json:"provider" yaml:"provider"`
	Model       string  `json:"model" yaml:"model"`
	Temperature float32 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
}

// AgentsConfig holds agent-related configuration.
type AgentsConfig struct {
	Coder    AgentConfig `json:"coder" yaml:"coder"`
	Tester   AgentConfig `json:"tester" yaml:"tester"`
	Reviewer AgentConfig `json:"reviewer" yaml:"reviewer"`
}

// AgentConfig holds configuration for a single agent.
type AgentConfig struct {
	Skills []string `json:"skills" yaml:"skills"`
	Model  string   `json:"model,omitempty" yaml:"model,omitempty"` // optional override
}

// SkillsConfig holds skill-related configuration.
type SkillsConfig struct {
	Knowledge map[string]string `json:"knowledge" yaml:"knowledge"`
	Tools     map[string]string `json:"tools" yaml:"tools"`
}

// RetryConfig holds retry-related configuration.
type RetryConfig struct {
	MaxRetries  int `json:"max_retries" yaml:"max_retries"`
	BackoffBase int `json:"backoff_base" yaml:"backoff_base"`
	BackoffMax  int `json:"backoff_max" yaml:"backoff_max"`
}

// applyDefaults ensures all nested maps and slices are initialized.
func (c *Config) applyDefaults() {
	if c.Models.Providers == nil {
		c.Models.Providers = make(map[string]ProviderConfig)
	}
	if c.Models.Phases == nil {
		c.Models.Phases = make(map[string]PhaseModelConfig)
	}
	if c.Skills.Knowledge == nil {
		c.Skills.Knowledge = make(map[string]string)
	}
	if c.Skills.Tools == nil {
		c.Skills.Tools = make(map[string]string)
	}
	if c.Agents.Coder.Skills == nil {
		c.Agents.Coder.Skills = []string{}
	}
	if c.Agents.Tester.Skills == nil {
		c.Agents.Tester.Skills = []string{}
	}
	if c.Agents.Reviewer.Skills == nil {
		c.Agents.Reviewer.Skills = []string{}
	}
	if c.Retry.MaxRetries == 0 {
		c.Retry.MaxRetries = 3
	}
	if c.Retry.BackoffBase == 0 {
		c.Retry.BackoffBase = 1000
	}
	if c.Retry.BackoffMax == 0 {
		c.Retry.BackoffMax = 30000
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
}

// LoadConfig reads configuration from a YAML file and applies defaults for missing fields.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	cfg.applyDefaults()
	return &cfg, nil
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

	cfg.applyDefaults()
	return &cfg, nil
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.Models.ActiveProvider == "" {
		return fmt.Errorf("active_provider is required")
	}

	if _, ok := c.Models.Providers[c.Models.ActiveProvider]; !ok {
		return fmt.Errorf("active_provider %q not found in providers", c.Models.ActiveProvider)
	}

	if len(c.Models.Phases) == 0 {
		return fmt.Errorf("at least one phase configuration is required")
	}

	for name, p := range c.Models.Providers {
		if p.BaseURL == "" {
			return fmt.Errorf("provider %q base_url is required", name)
		}
	}

	return nil
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
