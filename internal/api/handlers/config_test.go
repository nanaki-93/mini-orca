package handlers

import "github.com/nanaki-93/mini-orca/v2/internal/config"

func scopedHandlerConfig(apiBaseURL string) *config.Config {
	cfg := config.Default()
	cfg.ModelScopes.Analyze.APIBaseURL = apiBaseURL
	cfg.ModelScopes.Bug.APIBaseURL = apiBaseURL
	cfg.ModelScopes.Function.APIBaseURL = apiBaseURL
	return cfg
}
