package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/agent"
	"github.com/nanaki-93/mini-orca/v2/internal/agent/skills"
	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
)

// SkillsHandler manages HTTP handlers for skills lifecycle operations.
type SkillsHandler struct {
	store         *SkillsStore
	agentRegistry *agent.Registry
}

// NewSkillsHandler creates a new SkillsHandler instance.
func NewSkillsHandler(store *SkillsStore, agentRegistry *agent.Registry) *SkillsHandler {
	return &SkillsHandler{
		store:         store,
		agentRegistry: agentRegistry,
	}
}

// ListSkills handles GET /api/skills
// Returns all skills grouped by category.
func (h *SkillsHandler) ListSkills(w http.ResponseWriter, r *http.Request) {
	skillsByCategory := h.store.ListSkills()

	response := make(map[string][]SkillResponse)
	total := 0
	for category, skillList := range skillsByCategory {
		skills := make([]SkillResponse, 0, len(skillList))
		for _, skill := range skillList {
			skills = append(skills, skillToResponse(skill))
		}
		response[category] = skills
		total += len(skills)
	}

	WriteJSON(w, http.StatusOK, SkillListResponse{
		Skills: response,
		Total:  total,
	})
}

// CreateSkill handles POST /api/skills
// Creates a new skill.
func (h *SkillsHandler) CreateSkill(w http.ResponseWriter, r *http.Request) {
	var req SkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteAppError(w, apperrors.BadRequest("invalid request body", "The request body could not be parsed as JSON.", err))
		return
	}

	if req.Name == "" {
		WriteAppError(w, apperrors.BadRequest("name is required", "A skill name must be provided.", nil))
		return
	}

	if req.Type == "" {
		WriteAppError(w, apperrors.BadRequest("type is required", "A skill type must be provided.", nil))
		return
	}

	if req.Category == "" {
		WriteAppError(w, apperrors.BadRequest("category is required", "A skill category must be provided.", nil))
		return
	}

	skill, err := h.store.RegisterSkill(req)
	if err != nil {
		WriteAppError(w, apperrors.BadRequest("skill registration failed", "Failed to register new skill: "+err.Error(), err))
		return
	}

	WriteJSON(w, http.StatusCreated, SkillResponse{
		Name:           skill.Name,
		Type:           string(skill.Type),
		Category:       string(skill.Category),
		Description:    skill.Description,
		PromptTemplate: skill.PromptTemplate,
		Parameters:     skill.Parameters,
	})
}

// UpdateSkill handles PUT /api/skills/:id
// Updates an existing skill.
func (h *SkillsHandler) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	skillName := extractSkillID(r.URL.Path)
	if skillName == "" {
		WriteAppError(w, apperrors.BadRequest("skill ID is required", "A skill ID must be provided in the URL.", nil))
		return
	}

	// Check if skill exists
	_, err := h.store.GetSkill(skillName)
	if err != nil {
		WriteAppError(w, apperrors.NotFound("skill not found", "The specified skill could not be found.", err))
		return
	}

	var req SkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteAppError(w, apperrors.BadRequest("invalid request body", "The request body could not be parsed as JSON.", err))
		return
	}

	if req.Type == "" {
		WriteAppError(w, apperrors.BadRequest("type is required", "A skill type must be provided.", nil))
		return
	}

	if req.Category == "" {
		WriteAppError(w, apperrors.BadRequest("category is required", "A skill category must be provided.", nil))
		return
	}

	updatedSkill, err := h.store.RegisterSkill(req)
	if err != nil {
		WriteAppError(w, apperrors.BadRequest("skill update failed", "Failed to update skill: "+err.Error(), err))
		return
	}

	WriteJSON(w, http.StatusOK, SkillResponse{
		Name:           updatedSkill.Name,
		Type:           string(updatedSkill.Type),
		Category:       string(updatedSkill.Category),
		Description:    updatedSkill.Description,
		PromptTemplate: updatedSkill.PromptTemplate,
		Parameters:     updatedSkill.Parameters,
	})
}

// DeleteSkill handles DELETE /api/skills/:id
// Deletes a skill.
func (h *SkillsHandler) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	skillName := extractSkillID(r.URL.Path)
	if skillName == "" {
		WriteAppError(w, apperrors.BadRequest("skill ID is required", "A skill ID must be provided in the URL.", nil))
		return
	}

	if err := h.store.DeleteSkill(skillName); err != nil {
		WriteAppError(w, apperrors.NotFound("skill not found", "Could not delete skill: "+err.Error(), err))
		return
	}

	WriteJSON(w, http.StatusOK, SkillDeleteResponse{
		Message: fmt.Sprintf("skill %q deleted successfully", skillName),
	})
}

// ExportSkills handles GET /api/skills/export
// Exports all skills as JSON.
func (h *SkillsHandler) ExportSkills(w http.ResponseWriter, r *http.Request) {
	skills := h.store.ExportSkills()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=skills-export.json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(skills)
}

// ImportSkills handles POST /api/skills/import
// Imports skills from an uploaded JSON file.
func (h *SkillsHandler) ImportSkills(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB limit
		WriteAppError(w, apperrors.BadRequest("invalid form data", "Failed to parse multipart form data.", err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		WriteAppError(w, apperrors.BadRequest("file is required", "Please upload a skill export file.", err))
		return
	}
	defer file.Close()

	if header.Size == 0 {
		WriteAppError(w, apperrors.BadRequest("file is empty", "The uploaded file is empty.", nil))
		return
	}

	// Decode JSON
	var imported map[string]skills.Skill
	if err := json.NewDecoder(file).Decode(&imported); err != nil {
		WriteAppError(w, apperrors.BadRequest("invalid JSON format", "The uploaded file is not a valid skills JSON.", err))
		return
	}

	count := h.store.ImportSkills(imported)

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":  fmt.Sprintf("imported %d new skills", count),
		"imported": count,
	})
}

// GetAgentSkills handles GET /api/agents/:id/skills
// Returns the skills assigned to an agent and available skills.
func (h *SkillsHandler) GetAgentSkills(w http.ResponseWriter, r *http.Request) {
	agentName := extractAgentID(r.URL.Path)
	if agentName == "" {
		WriteAppError(w, apperrors.BadRequest("agent ID is required", "An agent ID must be provided in the URL.", nil))
		return
	}

	// Check if agent exists
	_, err := h.agentRegistry.Get(agentName)
	if err != nil {
		WriteAppError(w, apperrors.NotFound("agent not found", "The specified agent could not be found.", err))
		return
	}

	assignedSkills, err := h.store.GetAgentSkills(agentName)
	if err != nil {
		WriteAppError(w, apperrors.NotFound("skills not found", "Could not retrieve skills for the specified agent.", err))
		return
	}

	availableSkills := h.store.GetAvailableSkills(agentName)

	response := AgentSkillsResponse{
		AgentName:       agentName,
		AssignedSkills:  assignedSkills,
		AvailableSkills: make([]SkillResponse, 0, len(availableSkills)),
	}

	for _, skill := range availableSkills {
		response.AvailableSkills = append(response.AvailableSkills, skillToResponse(skill))
	}

	WriteJSON(w, http.StatusOK, response)
}

// SetAgentSkills handles PUT /api/agents/:id/skills
// Updates the skills assigned to an agent.
func (h *SkillsHandler) SetAgentSkills(w http.ResponseWriter, r *http.Request) {
	agentName := extractAgentID(r.URL.Path)
	if agentName == "" {
		WriteAppError(w, apperrors.BadRequest("agent ID is required", "An agent ID must be provided in the URL.", nil))
		return
	}

	// Check if agent exists
	_, err := h.agentRegistry.Get(agentName)
	if err != nil {
		WriteAppError(w, apperrors.NotFound("agent not found", "The specified agent could not be found.", err))
		return
	}

	var req AgentSkillsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteAppError(w, apperrors.BadRequest("invalid request body", "The request body could not be parsed as JSON.", err))
		return
	}

	if err := h.store.SetAgentSkills(agentName, req.Skills); err != nil {
		WriteAppError(w, apperrors.BadRequest("agent skills update failed", "Failed to update skills for the agent: "+err.Error(), err))
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":    fmt.Sprintf("agent %q skills updated", agentName),
		"agent_name": agentName,
		"skills":     req.Skills,
		"updated_at": time.Now(),
	})
}

// ResetSkills handles POST /api/skills/reset
// Resets all skills to their default state.
func (h *SkillsHandler) ResetSkills(w http.ResponseWriter, r *http.Request) {
	h.store.ResetSkills()

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "skills reset to defaults",
		"reset_at": time.Now(),
	})
}
