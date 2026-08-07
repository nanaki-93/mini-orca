// Package handlers provides HTTP handlers for the Mini-Orca REST API.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
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

// ConfigResponse represents the response body for configuration data.
type ConfigResponse struct {
	// LLM holds LLM configuration.
	LLM config.LLMConfig `json:"llm"`
	// Agents holds agent-related configuration.
	Agents config.AgentsConfig `json:"agents"`
	// Retry holds retry-related configuration.
	Retry config.RetryConfig `json:"retry"`
}

// ─── Store ────────────────────────────────────────────────────────────────────

// ConfigStore provides thread-safe access to application configuration.
type ConfigStore struct {
	mu   sync.RWMutex
	cfg  *config.Config
	path string // config file path for persistence
}

// NewConfigStore creates a new ConfigStore with the given configuration and file path.
func NewConfigStore(cfg *config.Config, path string) *ConfigStore {
	return &ConfigStore{
		cfg:  cfg,
		path: path,
	}
}

// GetConfig returns a copy of the current configuration.
func (s *ConfigStore) GetConfig() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return copyConfig(s.cfg)
}

// UpdateConfig applies updates to the configuration and persists to disk.
func (s *ConfigStore) UpdateConfig(req *ConfigUpdateRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.applyUpdates(req); err != nil {
		return err
	}

	if err := s.cfg.Validate(); err != nil {
		return fmt.Errorf("config update failed validation: %w", err)
	}

	// Persist to disk
	if s.path != "" {
		if err := s.cfg.Save(s.path); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	return nil
}

// LoadConfig loads configuration from the file path.
func LoadConfig(path string) (*config.Config, error) {
	return config.LoadFromYAML(path)
}

// applyUpdates applies the configuration updates to the store's config.
func (s *ConfigStore) applyUpdates(req *ConfigUpdateRequest) error {
	if req == nil {
		return nil
	}

	if req.LLM != nil {
		if err := s.updateLLM(req.LLM); err != nil {
			return err
		}
	}

	if req.Agents != nil {
		if err := s.updateAgents(req.Agents); err != nil {
			return err
		}
	}

	if req.Retry != nil {
		if err := s.updateRetry(req.Retry); err != nil {
			return err
		}
	}

	return nil
}

// updateLLM applies LLM configuration updates.
func (s *ConfigStore) updateLLM(llm *config.LLMConfig) error {
	if llm.BaseURL != "" {
		s.cfg.LLM.BaseURL = llm.BaseURL
	}
	if llm.APIKey != "" {
		s.cfg.LLM.APIKey = llm.APIKey
	}
	if llm.Model != "" {
		s.cfg.LLM.Model = llm.Model
	}
	if llm.Temperature > 0 {
		s.cfg.LLM.Temperature = llm.Temperature
	}
	if llm.MaxTokens > 0 {
		s.cfg.LLM.MaxTokens = llm.MaxTokens
	}
	return nil
}

// updateAgents applies agent configuration updates.
func (s *ConfigStore) updateAgents(agents *config.AgentsConfig) error {

	if agents.Coder.Skills != nil {
		s.cfg.Agents.Coder.Skills = agents.Coder.Skills
	}
	if agents.Coder.Model != "" {
		s.cfg.Agents.Coder.Model = agents.Coder.Model
	}

	if agents.Tester.Skills != nil {
		s.cfg.Agents.Tester.Skills = agents.Tester.Skills
	}
	if agents.Tester.Model != "" {
		s.cfg.Agents.Tester.Model = agents.Tester.Model
	}

	if agents.Reviewer.Skills != nil {
		s.cfg.Agents.Reviewer.Skills = agents.Reviewer.Skills
	}
	if agents.Reviewer.Model != "" {
		s.cfg.Agents.Reviewer.Model = agents.Reviewer.Model
	}

	return nil
}

// updateRetry applies retry configuration updates.
func (s *ConfigStore) updateRetry(retry *config.RetryConfig) error {
	if retry.MaxRetries > 0 {
		s.cfg.Retry.MaxRetries = retry.MaxRetries
	}
	if retry.BackoffBase > 0 {
		s.cfg.Retry.BackoffBase = retry.BackoffBase
	}
	if retry.BackoffMax > 0 {
		s.cfg.Retry.BackoffMax = retry.BackoffMax
	}

	return nil
}

// copyConfig creates a deep copy of the configuration.
func copyConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return nil
	}

	cfgCopy := *cfg

	// LLM is a value type, no deep copy needed

	// Deep copy agent skills
	cfgCopy.Agents.Coder.Skills = make([]string, len(cfg.Agents.Coder.Skills))
	copy(cfgCopy.Agents.Coder.Skills, cfg.Agents.Coder.Skills)
	cfgCopy.Agents.Tester.Skills = make([]string, len(cfg.Agents.Tester.Skills))
	copy(cfgCopy.Agents.Tester.Skills, cfg.Agents.Tester.Skills)
	cfgCopy.Agents.Reviewer.Skills = make([]string, len(cfg.Agents.Reviewer.Skills))
	copy(cfgCopy.Agents.Reviewer.Skills, cfg.Agents.Reviewer.Skills)

	return &cfgCopy
}

// ─── Handler ──────────────────────────────────────────────────────────────────

// ConfigHandler manages HTTP handlers for configuration operations.
type ConfigHandler struct {
	// configStore provides access to application configuration.
	configStore *ConfigStore
}

// NewConfigHandler creates a new ConfigHandler instance.
func NewConfigHandler(configStore *ConfigStore) *ConfigHandler {
	return &ConfigHandler{
		configStore: configStore,
	}
}

// GetConfig handles GET /api/config
// Returns the current configuration.
func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.configStore.GetConfig()

	api.WriteJSON(w, http.StatusOK, ConfigResponse{
		LLM:    cfg.LLM,
		Agents: cfg.Agents,
		Retry:  cfg.Retry,
	})
}

// UpdateConfig handles PUT /api/config
// Updates the configuration with the provided values.
func (h *ConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteAppError(w, apperrors.BadRequest("invalid request body", "The request body could not be parsed as JSON.", err))
		return
	}

	if err := h.configStore.UpdateConfig(&req); err != nil {
		api.WriteAppError(w, apperrors.BadRequest("config update failed", "Failed to update configuration: "+err.Error(), err))
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "updated",
		"message": "configuration updated successfully",
	})
}

// ─── Config File Utilities ────────────────────────────────────────────────────

// ConfigExists checks if a config file exists at the given path.
func ConfigExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DefaultConfigPath returns the default configuration file path.
func DefaultConfigPath() string {
	return "config.yaml"
}
