package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nanaki-93/mini-orca/v2/internal/logging"
	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration struct.
type Config struct {
	LLM         LLMConfig     `json:"llm" yaml:"llm"`
	Agents      AgentsConfig  `json:"agents" yaml:"agents"`
	Retry       RetryConfig   `json:"retry" yaml:"retry"`
	Timeouts    TimeoutConfig `json:"timeouts" yaml:"timeouts"`
	Logging     LoggingConfig `json:"logging" yaml:"logging"`
	ProjectPath string        `json:"project_path" yaml:"project_path"`
}

// LoggingConfig holds configuration for the logger.
type LoggingConfig struct {
	Level         string   `json:"level" yaml:"level"`
	Format        string   `json:"format" yaml:"format"`
	Filename      string   `json:"filename" yaml:"filename"`
	SensitiveKeys []string `json:"sensitive_keys" yaml:"sensitive_keys"`
}

// LLMConfig holds the flat LLM configuration.
type LLMConfig struct {
	BaseURL     string  `json:"base_url" yaml:"base_url"`
	APIKey      string  `json:"api_key,omitempty" yaml:"api_key,omitempty"`
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
	Skills         []string `json:"skills" yaml:"skills"`
	Model          string   `json:"model,omitempty" yaml:"model,omitempty"` // optional override
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
}

// RetryConfig holds retry-related configuration.
type RetryConfig struct {
	MaxRetries  int `json:"max_retries" yaml:"max_retries"`
	BackoffBase int `json:"backoff_base" yaml:"backoff_base"`
	BackoffMax  int `json:"backoff_max" yaml:"backoff_max"`
}

// TimeoutConfig sets operation deadlines independently of HTTP server timeouts.
// Zero values use conservative local-daemon defaults.
type TimeoutConfig struct {
	ImportSeconds       int `json:"import_seconds" yaml:"import_seconds"`
	AnalysisSeconds     int `json:"analysis_seconds" yaml:"analysis_seconds"`
	GenerationSeconds   int `json:"generation_seconds" yaml:"generation_seconds"`
	FocusedCheckSeconds int `json:"focused_check_seconds" yaml:"focused_check_seconds"`
}

// applyDefaults ensures all nested maps and slices are initialized.
func (c *Config) applyDefaults() {
	if c.LLM.BaseURL == "" {
		c.LLM.BaseURL = DefaultProviderURL
	}
	if c.LLM.Model == "" {
		c.LLM.Model = ""
	}
	if c.LLM.Temperature == 0 {
		c.LLM.Temperature = 0.7
	}
	if c.LLM.MaxTokens == 0 {
		c.LLM.MaxTokens = 8192
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
	if c.Timeouts.ImportSeconds == 0 {
		c.Timeouts.ImportSeconds = 300
	}
	if c.Timeouts.AnalysisSeconds == 0 {
		c.Timeouts.AnalysisSeconds = 300
	}
	if c.Timeouts.GenerationSeconds == 0 {
		c.Timeouts.GenerationSeconds = c.Agents.Coder.TimeoutSeconds
		if c.Timeouts.GenerationSeconds == 0 {
			c.Timeouts.GenerationSeconds = 300
		}
	}
	if c.Timeouts.FocusedCheckSeconds == 0 {
		c.Timeouts.FocusedCheckSeconds = 60
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
	if c.ProjectPath == "" {
		if wd, err := os.Getwd(); err == nil {
			c.ProjectPath = wd
		} else {
			c.ProjectPath = "."
		}
	}
}

// LoadConfig reads configuration from the specified path or uses defaults.
func LoadConfig() (*Config, error) {
	configPath := "config.yaml"
	if p := os.Getenv("MINI_ORCA_CONFIG"); p != "" {
		configPath = p
	}

	cfg, err := LoadFromYAML(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Info("Config file not found, using defaults", "path", configPath)
			return Default(), nil
		}
		return nil, err
	}

	logging.Info("Loaded config", "path", configPath)
	return cfg, nil
}

// LoadFromYAML reads configuration from a YAML file and applies defaults for missing fields.
func LoadFromYAML(path string) (*Config, error) {
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

// LoadFromJSON reads configuration from a JSON file.
func LoadFromJSON(path string) (*Config, error) {
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
	if c.LLM.BaseURL == "" {
		return fmt.Errorf("llm.base_url is required")
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
