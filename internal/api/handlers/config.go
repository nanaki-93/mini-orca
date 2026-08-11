// Package handlers provides HTTP handlers for the Mini-Orca REST API.
package handlers

import (
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

// ─── Request/Response Types ──────────────────────────────────────────────────

// ConfigUpdateRequest represents the request body for updating configuration.
type ConfigUpdateRequest struct {
	// LLM holds LLM configuration updates.
	LLM *config.LLMConfig `json:"llm,omitempty"`
	// Agents holds agent-related configuration updates.
	Agents *config.AgentsConfig `json:"agents,omitempty"`
	// Retry holds retry-related configuration updates.
	Retry *config.RetryConfig `json:"retry,omitempty"`
}
