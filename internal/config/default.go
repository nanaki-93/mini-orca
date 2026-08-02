package config

// DefaultProviderURL is the default LM Studio URL.
const DefaultProviderURL = "http://localhost:1234"

// DefaultPhaseConfigs returns a map of phase names to their default PhaseModelConfig.
func DefaultPhaseConfigs() map[string]PhaseModelConfig {
	return map[string]PhaseModelConfig{
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
	}
}

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

// DefaultSkills returns predefined skill definitions with descriptions.
func DefaultSkills() SkillsConfig {
	return SkillsConfig{
		Knowledge: map[string]string{
			"task_breakdown":      "Break down complex tasks into manageable subtasks",
			"context_analysis":    "Analyze project context and requirements",
			"plan_generation":     "Generate structured execution plans",
			"code_generation":     "Generate clean, well-documented code",
			"refactoring":         "Refactor code for clarity and maintainability",
			"debugging":           "Debug and fix issues in code",
			"test_generation":     "Generate comprehensive unit and integration tests",
			"edge_case_detection": "Identify and handle edge cases",
			"validation":          "Validate code against requirements and standards",
			"code_review":         "Review code for quality and correctness",
			"security_check":      "Identify potential security vulnerabilities",
			"best_practices":      "Enforce coding best practices and patterns",
		},
		Tools: map[string]string{
			"formatter":   "Format code according to project standards",
			"linter":      "Lint code for style and correctness",
			"test_runner": "Execute test suites",
		},
	}
}

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
			Phases: DefaultPhaseConfigs(),
		},
		Agents: DefaultAgentConfigs(),
		Skills: DefaultSkills(),
	}
}
