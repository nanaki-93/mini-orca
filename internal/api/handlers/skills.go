package handlers

import (
	"encoding/json"
	"net/http"

	"mini-orca/internal/agent/skills"
	"mini-orca/internal/types"
)

// SkillsHandler handles skills-related API endpoints.
type SkillsHandler struct {
	skillRegistry *skills.Registry
}

// NewSkillsHandler creates a new skills handler.
func NewSkillsHandler(registry *skills.Registry) *SkillsHandler {
	return &SkillsHandler{
		skillRegistry: registry,
	}
}

// ListSkills handles GET /api/skills
func (h *SkillsHandler) ListSkills(w http.ResponseWriter, r *http.Request) {
	skillList := h.skillRegistry.List()
	
	response := make([]map[string]interface{}, 0)
	for _, skill := range skillList {
		response = append(response, map[string]interface{}{
			"id":          skill.ID,
			"name":        skill.Name,
			"description": skill.Description,
			"type":        skill.Type,
			"priority":    skill.Priority,
			"enabled":     skill.Enabled,
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetSkill handles GET /api/skills/:id
func (h *SkillsHandler) GetSkill(w http.ResponseWriter, r *http.Request) {
	// In production, parse ID from URL
	skillID := r.URL.Query().Get("id")
	
	skill, err := h.skillRegistry.Get(skillID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(skill)
}

// CreateSkill handles POST /api/skills
func (h *SkillsHandler) CreateSkill(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Priority    int    `json:"priority"`
		Prompt      string `json:"prompt"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	skill := skills.NewSkill(req.Name, req.Description, req.Type)
	skill.Priority = req.Priority
	skill.Prompt = req.Prompt
	skill.Type = types.SkillType(req.Type)
	
	if err := h.skillRegistry.Add(skill); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "created",
		"skill":   skill,
	})
}

// UpdateSkill handles PUT /api/skills/:id
func (h *SkillsHandler) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	skillID := r.URL.Query().Get("id")
	
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Priority    int    `json:"priority"`
		Prompt      string `json:"prompt"`
		Enabled     bool   `json:"enabled"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	skill, err := h.skillRegistry.Get(skillID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	skill.Name = req.Name
	skill.Description = req.Description
	skill.Type = types.SkillType(req.Type)
	skill.Priority = req.Priority
	skill.Prompt = req.Prompt
	skill.Enabled = req.Enabled
	
	if err := h.skillRegistry.Update(skill); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "updated",
		"skill":  skill,
	})
}

// DeleteSkill handles DELETE /api/skills/:id
func (h *SkillsHandler) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	skillID := r.URL.Query().Get("id")
	
	if err := h.skillRegistry.Remove(skillID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "deleted",
		"id":     skillID,
	})
}

// GetAgentSkills handles GET /api/agents/:agent/skills
func (h *SkillsHandler) GetAgentSkills(w http.ResponseWriter, r *http.Request) {
	agent := r.URL.Query().Get("agent")
	
	skillList := h.skillRegistry.List()
	response := make([]map[string]interface{}, 0)
	
	for _, skill := range skillList {
		assigned := h.skillRegistry.IsAgentSkill(agent, skill.ID)
		response = append(response, map[string]interface{}{
			"id":       skill.ID,
			"name":     skill.Name,
			"assigned": assigned,
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AssignAgentSkill handles POST /api/agents/:agent/skills/:skill
func (h *SkillsHandler) AssignAgentSkill(w http.ResponseWriter, r *http.Request) {
	agent := r.URL.Query().Get("agent")
	skillID := r.URL.Query().Get("skill")
	
	if err := h.skillRegistry.AssignAgentSkill(agent, skillID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "assigned",
		"agent":  agent,
		"skill":  skillID,
	})
}

// BulkAssignSkills handles POST /api/skills/bulk-assign
func (h *SkillsHandler) BulkAssignSkills(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Agent    string   `json:"agent"`
		SkillIDs []string `json:"skill_ids"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	for _, skillID := range req.SkillIDs {
		h.skillRegistry.AssignAgentSkill(req.Agent, skillID)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "assigned",
		"agent":  req.Agent,
		"count":  len(req.SkillIDs),
	})
}

// ExportSkills handles GET /api/skills/export
func (h *SkillsHandler) ExportSkills(w http.ResponseWriter, r *http.Request) {
	skillList := h.skillRegistry.List()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(skillList)
}

// ImportSkills handles POST /api/skills/import
func (h *SkillsHandler) ImportSkills(w http.ResponseWriter, r *http.Request) {
	var skillList []skills.Skill
	
	if err := json.NewDecoder(r.Body).Decode(&skillList); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	for _, skill := range skillList {
		h.skillRegistry.Add(&skill)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "imported",
		"count":  len(skillList),
	})
}
