package config

import (
	"github.com/nanaki-93/mini-orca/internal/model"
)

// DefaultProviderURL is the default LM Studio URL.
const DefaultProviderURL = "http://localhost:1234"

// DefaultModelID is the default model ID when none is specified.
const DefaultModelID = ""

// DefaultTemperature is the default temperature for chat completions.
const DefaultTemperature float32 = 0.7

// DefaultMaxTokens is the default max tokens for chat completions.
const DefaultMaxTokens = 4096

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Config: model.Config{
			ActiveProvider: "lm-studio",
			Providers: map[string]model.ProviderConfig{
				"lm-studio": {
					Name:    "lm-studio",
					BaseURL: DefaultProviderURL,
				},
			},
			Phases: map[model.Phase]model.PhaseConfig{
				model.PhasePlanning: {
					ModelConfig: model.ModelConfig{
						Provider: "lm-studio",
						ModelID:  DefaultModelID,
					},
					Temperature: DefaultTemperature,
					MaxTokens:   DefaultMaxTokens,
				},
				model.PhaseCoding: {
					ModelConfig: model.ModelConfig{
						Provider: "lm-studio",
						ModelID:  DefaultModelID,
					},
					Temperature: DefaultTemperature,
					MaxTokens:   DefaultMaxTokens,
				},
				model.PhaseTesting: {
					ModelConfig: model.ModelConfig{
						Provider: "lm-studio",
						ModelID:  DefaultModelID,
					},
					Temperature: DefaultTemperature,
					MaxTokens:   DefaultMaxTokens,
				},
				model.PhaseReview: {
					ModelConfig: model.ModelConfig{
						Provider: "lm-studio",
						ModelID:  DefaultModelID,
					},
					Temperature: DefaultTemperature,
					MaxTokens:   DefaultMaxTokens,
				},
				model.PhaseHumanReview: {
					ModelConfig: model.ModelConfig{
						Provider: "lm-studio",
						ModelID:  DefaultModelID,
					},
					Temperature: DefaultTemperature,
					MaxTokens:   DefaultMaxTokens,
				},
			},
			Agents: make(map[string]model.AgentConfig),
		},
	}
}
