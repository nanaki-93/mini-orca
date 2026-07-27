package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nanaki-93/mini-orca/internal/model"
)

// Config wraps the model.Config and provides file loading/saving.
type Config struct {
	model.Config
}

// Load reads configuration from a JSON file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg model.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Initialize maps to avoid nil map issues
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]model.ProviderConfig)
	}
	if cfg.Phases == nil {
		cfg.Phases = make(map[model.Phase]model.PhaseConfig)
	}
	if cfg.Agents == nil {
		cfg.Agents = make(map[string]model.AgentConfig)
	}

	return &Config{Config: cfg}, nil
}

// Save writes configuration to a JSON file.
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c.Config, "", "  ")
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
	if c.ActiveProvider == "" {
		return fmt.Errorf("active_provider is required")
	}

	if _, ok := c.Providers[c.ActiveProvider]; !ok {
		return fmt.Errorf("active_provider %q not found in providers", c.ActiveProvider)
	}

	return nil
}
