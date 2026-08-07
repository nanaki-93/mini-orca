package config

// DefaultProviderURL is the default LM Studio URL.
const DefaultProviderURL = "http://localhost:1234"

// DefaultAgentConfigs returns default agent configurations with predefined skill lists.
func DefaultAgentConfigs() AgentsConfig {
	return AgentsConfig{
		Coder: AgentConfig{
			Skills: []string{"code_generation", "refactoring", "debugging"},
		},
		Tester: AgentConfig{
			Skills: []string{"test_generation", "edge_case_detection", "validation"},
		},
		Reviewer: AgentConfig{
			Skills: []string{"code_review", "security_check", "best_practices"},
		},
	}
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		LLM: LLMConfig{
			BaseURL:     DefaultProviderURL,
			Model:       "",
			Temperature: 0.7,
			MaxTokens:   8192,
		},
		Agents: DefaultAgentConfigs(),
	}
}
