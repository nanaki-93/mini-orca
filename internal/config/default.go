package config

// DefaultProviderAPIBaseURL is the local LM Studio OpenAI-compatible endpoint.
const DefaultProviderAPIBaseURL = "http://localhost:1234/v1"

// Default returns explicit local scoped profiles. Users should replace the
// placeholder model IDs in their ignored config.yaml before making requests.
func Default() *Config {
	cfg := &Config{
		ModelScopes: ModelScopesConfig{
			Analyze:  ModelProfileConfig{APIBaseURL: DefaultProviderAPIBaseURL, Model: "local-analysis-model"},
			Bug:      ModelProfileConfig{APIBaseURL: DefaultProviderAPIBaseURL, Model: "local-bug-model"},
			Function: ModelProfileConfig{APIBaseURL: DefaultProviderAPIBaseURL, Model: "local-function-model"},
		},
		Retry: RetryConfig{MaxRetries: 3, BackoffBase: 1000, BackoffMax: 30000},
		Timeouts: TimeoutConfig{
			ImportSeconds: 300, AnalysisSeconds: 300, GenerationSeconds: 300, FocusedCheckSeconds: 60,
		},
		Logging: LoggingConfig{Level: "info", Format: "json"},
	}
	cfg.applyDefaults()
	return cfg
}
