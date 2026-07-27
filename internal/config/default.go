package config

// DefaultProviderURL is the default LM Studio URL.
const DefaultProviderURL = "http://localhost:1234"

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Models: ModelsConfig{
			ActiveProvider: "lm-studio",
			Providers: map[string]ProviderConfig{
				"lm-studio": {
					BaseURL: DefaultProviderURL,
				},
			},
			Phases: map[string]PhaseModelConfig{
				"planning": {
					Provider:    "lm-studio",
					Model:       "",
					Temperature: 0.7,
					MaxTokens:   4096,
				},
				"coding": {
					Provider:    "lm-studio",
					Model:       "",
					Temperature: 0.7,
					MaxTokens:   4096,
				},
				"testing": {
					Provider:    "lm-studio",
					Model:       "",
					Temperature: 0.7,
					MaxTokens:   4096,
				},
				"review": {
					Provider:    "lm-studio",
					Model:       "",
					Temperature: 0.7,
					MaxTokens:   4096,
				},
				"human_review": {
					Provider:    "lm-studio",
					Model:       "",
					Temperature: 0.7,
					MaxTokens:   4096,
				},
			},
		},
		Agents: AgentsConfig{
			Planner:  AgentConfig{Skills: []string{}},
			Coder:    AgentConfig{Skills: []string{}},
			Tester:   AgentConfig{Skills: []string{}},
			Reviewer: AgentConfig{Skills: []string{}},
		},
		Skills: SkillsConfig{
			Knowledge: make(map[string]string),
			Tools:     make(map[string]string),
		},
	}
}
