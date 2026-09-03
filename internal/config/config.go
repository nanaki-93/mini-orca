package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/logging"
	"gopkg.in/yaml.v3"
)

// Config is the daemon's single YAML configuration schema.
type Config struct {
	ModelScopes ModelScopesConfig `yaml:"model_scopes"`
	Retry       RetryConfig       `yaml:"retry"`
	Timeouts    TimeoutConfig     `yaml:"timeouts"`
	Logging     LoggingConfig     `yaml:"logging"`
	ProjectPath string            `yaml:"project_path"`
}

// LoggingConfig holds configuration for the logger.
type LoggingConfig struct {
	Level         string   `yaml:"level"`
	Format        string   `yaml:"format"`
	Filename      string   `yaml:"filename"`
	SensitiveKeys []string `yaml:"sensitive_keys"`
}

// ModelScopesConfig contains the three fixed model profiles used by the daemon.
type ModelScopesConfig struct {
	Analyze  ModelProfileConfig `yaml:"analyze"`
	Bug      ModelProfileConfig `yaml:"bug"`
	Function ModelProfileConfig `yaml:"function"`
}

// ModelProfileConfig is the configured portion of a fixed scope. Pointer
// numeric fields preserve the difference between an omitted value and zero.
type ModelProfileConfig struct {
	APIBaseURL       string   `yaml:"api_base_url"`
	APIKey           string   `yaml:"api_key"`
	Model            string   `yaml:"model"`
	ReasoningEffort  string   `yaml:"reasoning_effort"`
	Temperature      *float32 `yaml:"temperature"`
	MaxTokens        *int     `yaml:"max_tokens"`
	ContextMaxTokens *int     `yaml:"context_max_tokens"`
}

// RetryConfig holds retry-related configuration.
type RetryConfig struct {
	MaxRetries  int `yaml:"max_retries"`
	BackoffBase int `yaml:"backoff_base"`
	BackoffMax  int `yaml:"backoff_max"`
}

// TimeoutConfig sets operation deadlines independently of HTTP server timeouts.
// Zero values use conservative local-daemon defaults.
type TimeoutConfig struct {
	ImportSeconds       int `yaml:"import_seconds"`
	AnalysisSeconds     int `yaml:"analysis_seconds"`
	GenerationSeconds   int `yaml:"generation_seconds"`
	FocusedCheckSeconds int `yaml:"focused_check_seconds"`
}

func (c *Config) applyDefaults() {
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
		c.Timeouts.GenerationSeconds = 300
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
		if workingDirectory, err := os.Getwd(); err == nil {
			c.ProjectPath = workingDirectory
		} else {
			c.ProjectPath = "."
		}
	}
}

// LoadConfig reads the configured YAML file or returns explicit local defaults
// when no configuration file exists.
func LoadConfig() (*Config, error) {
	configPath := "config.yaml"
	if path := os.Getenv("MINI_ORCA_CONFIG"); path != "" {
		configPath = path
	}

	cfg, err := LoadFromYAML(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Info("Config file not found, using local defaults", "path", configPath)
			return Default(), nil
		}
		return nil, err
	}

	logging.Info("Loaded config", "path", configPath)
	return cfg, nil
}

// LoadFromYAML reads the only supported configuration format. Unknown keys are
// rejected before a startup can silently accept retired configuration.
func LoadFromYAML(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}
	if err := rejectUnknownFields(data); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parse config file: configuration must contain one YAML document")
		}
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks that the configured scopes are complete and safe before any
// provider request can be constructed.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("configuration is required")
	}
	_, err := ResolveModelProfiles(c)
	return err
}

var configurationFields = fieldSet{
	"model_scopes": {
		"analyze":  modelProfileFields,
		"bug":      modelProfileFields,
		"function": modelProfileFields,
	},
	"retry": {
		"max_retries": {}, "backoff_base": {}, "backoff_max": {},
	},
	"timeouts": {
		"import_seconds": {}, "analysis_seconds": {}, "generation_seconds": {}, "focused_check_seconds": {},
	},
	"logging": {
		"level": {}, "format": {}, "filename": {}, "sensitive_keys": {},
	},
	"project_path": {},
}

var modelProfileFields = fieldSet{
	"api_base_url": {}, "api_key": {}, "model": {}, "reasoning_effort": {},
	"temperature": {}, "max_tokens": {}, "context_max_tokens": {},
}

type fieldSet map[string]fieldSet

func rejectUnknownFields(data []byte) error {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return err
	}
	if len(document.Content) == 0 {
		return nil
	}
	return rejectUnknownMapping(document.Content[0], "", configurationFields)
}

func rejectUnknownMapping(node *yaml.Node, prefix string, fields fieldSet) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		key := node.Content[index].Value
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		children, known := fields[key]
		if !known {
			return fmt.Errorf("unsupported configuration field %q", path)
		}
		if len(children) > 0 {
			if err := rejectUnknownMapping(node.Content[index+1], path, children); err != nil {
				return err
			}
		}
	}
	return nil
}

func missingScopeError(scope ModelScope) error {
	return fmt.Errorf("model_scopes.%s is required", scope)
}

func missingFieldError(scope ModelScope, field string) error {
	return fmt.Errorf("model_scopes.%s.%s is required", scope, strings.TrimSpace(field))
}
