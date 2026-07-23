package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"mini-orca/internal/model"
)

const defaultConfigName = "config.yaml"

// Config holds the complete application configuration.
type Config struct {
	Models model.Config `yaml:"models"`
	Agents AgentConfig  `yaml:"agents"`
	Server ServerConfig `yaml:"server"`
}

// AgentConfig holds agent-specific configuration.
type AgentConfig struct {
	Planner  AgentSettings `yaml:"planner"`
	Coder    AgentSettings `yaml:"coder"`
	Tester   AgentSettings `yaml:"tester"`
	Reviewer AgentSettings `yaml:"reviewer"`
}

// AgentSettings holds settings for a single agent.
type AgentSettings struct {
	Skills []string `yaml:"skills"`
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	Port int `yaml:"port"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Models: model.Config{
			ActiveProvider: "lm-studio",
			Providers: map[string]model.ProviderConfig{
				"lm-studio": {
					BaseURL: "http://127.0.0.1:1234",
				},
			},
			Phases: map[model.Phase]model.PhaseConfig{
				model.PhasePlanning: {
					Provider:    "lm-studio",
					Model:       "qwen/qwen3-coder-30b",
					Temperature: 0.3,
					MaxTokens:   8192,
				},
				model.PhaseCoding: {
					Provider:    "lm-studio",
					Model:       "qwen/qwen3-coder-30b",
					Temperature: 0.1,
					MaxTokens:   8192,
				},
				model.PhaseTesting: {
					Provider:    "lm-studio",
					Model:       "qwen/qwen3-coder-30b",
					Temperature: 0.2,
					MaxTokens:   8192,
				},
				model.PhaseReview: {
					Provider:    "lm-studio",
					Model:       "qwen/qwen3-coder-30b",
					Temperature: 0.2,
					MaxTokens:   8192,
				},
			},
		},
		Agents: AgentConfig{
			Planner: AgentSettings{
				Skills: []string{
					"solid_principles",
					"clean_code",
					"kiss_principle",
					"business_logic_adherence",
					"architecture_design",
					"task_breakdown",
					"dependency_mapping",
				},
			},
			Coder: AgentSettings{
				Skills: []string{
					"solid_principles",
					"clean_code",
					"kiss_principle",
					"no_repetition",
					"function_generation",
					"struct_design",
					"class_creation",
				},
			},
			Tester: AgentSettings{
				Skills: []string{
					"clean_code",
					"unit_testing",
					"integration_testing",
					"coverage_analysis",
					"test_generation",
				},
			},
			Reviewer: AgentSettings{
				Skills: []string{
					"solid_principles",
					"clean_code",
					"business_logic_adherence",
					"style_check",
					"logic_review",
					"security_audit",
				},
			},
		},
		Server: ServerConfig{
			Port: 8080,
		},
	}
}

// Load loads configuration from a file.
// If the file doesn't exist, it creates it with defaults.
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	// Try to load from file
	if configPath == "" {
		// Try default locations
		locations := []string{
			"config/config.yaml",
			".mini-orca/config.yaml",
		}
		for _, loc := range locations {
			if _, err := os.Stat(loc); err == nil {
				configPath = loc
				break
			}
		}
	}

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Create default config file
				if err := Save(cfg, configPath); err != nil {
					return nil, fmt.Errorf("default config not found and could not create one: %w", err)
				}
				return &cfg, nil
			}
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	return &cfg, nil
}

// Save writes the configuration to a file.
func Save(cfg Config, path string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// BuildRouter creates a model Router from the configuration.
func (c *Config) BuildRouter() (*model.Router, error) {
	router := model.NewRouter(c.Models)

	// Register LM Studio provider
	if lmCfg, ok := c.Models.Providers["lm-studio"]; ok {
		provider := model.NewLMStudioProvider(lmCfg.BaseURL, lmCfg.APIKey)
		router.RegisterProvider("lm-studio", provider)
	}

	return router, nil
}
