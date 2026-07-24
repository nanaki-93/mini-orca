package handlers

import (
	"encoding/json"
	"net/http"

	"mini-orca/internal/config"
	"mini-orca/internal/model"
)

// ConfigHandler handles configuration-related API endpoints.
type ConfigHandler struct {
	config     *config.Config
	router     *model.Router
}

// NewConfigHandler creates a new config handler.
func NewConfigHandler(cfg *config.Config, router *model.Router) *ConfigHandler {
	return &ConfigHandler{
		config: cfg,
		router: router,
	}
}

// GetConfig handles GET /api/config
func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"models": map[string]interface{}{
			"active_provider": h.config.Models.ActiveProvider,
			"providers":       h.config.Models.Providers,
			"phases":          h.config.Models.Phases,
		},
		"agents": map[string]interface{}{
			"planner":  h.config.Agents.Planner.Skills,
			"coder":    h.config.Agents.Coder.Skills,
			"tester":   h.config.Agents.Tester.Skills,
			"reviewer": h.config.Agents.Reviewer.Skills,
		},
		"server": map[string]interface{}{
			"port": h.config.Server.Port,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateConfig handles POST /api/config
func (h *ConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Update config based on request
	// In production, validate and save to file
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "updated",
		"message": "Configuration updated successfully",
	})
}

// GetAvailableModels handles GET /api/models
func (h *ConfigHandler) GetAvailableModels(w http.ResponseWriter, r *http.Request) {
	phase := r.URL.Query().Get("phase")
	
	// In production, query the provider for available models
	availableModels := []string{
		"qwen/qwen3-coder-30b",
		"mistral/mistral-7b",
		"llama/llama-2-70b",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"phase":           phase,
		"available_models": availableModels,
	})
}
