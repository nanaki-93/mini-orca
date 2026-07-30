// Package api provides HTTP handlers for the Mini-Orca REST API.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/internal/agent"
	"github.com/nanaki-93/mini-orca/internal/agent/skills"
	apperrors "github.com/nanaki-93/mini-orca/internal/errors"
)

// SkillRequest represents the request body for creating or updating a skill.
type SkillRequest struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Category       string            `json:"category"`
	Description    string            `json:"description"`
	PromptTemplate string            `json:"prompt_template"`
	Parameters     map[string]string `json:"parameters,omitempty"`
}

// SkillResponse represents the response body for skill data.
type SkillResponse struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Category       string            `json:"category"`
	Description    string            `json:"description"`
	PromptTemplate string            `json:"prompt_template"`
	Parameters     map[string]string `json:"parameters,omitempty"`
}

// SkillListResponse represents the response body for listing skills.
type SkillListResponse struct {
	Skills map[string][]SkillResponse `json:"skills"`
	Total  int                        `json:"total"`
}

// SkillSearchResponse represents the response body for skill search results.
type SkillSearchResponse struct {
	Results []SkillResponse `json:"results"`
	Total   int             `json:"total"`
}

// SkillDeleteResponse represents the response body for skill deletion.
type SkillDeleteResponse struct {
	Message string `json:"message"`
}

// AgentSkillsResponse represents the response body for agent skills.
type AgentSkillsResponse struct {
	AgentName       string          `json:"agent_name"`
	AssignedSkills  []string        `json:"assigned_skills"`
	AvailableSkills []SkillResponse `json:"available_skills"`
}

// AgentSkillsUpdateRequest represents the request body for updating agent skills.
type AgentSkillsUpdateRequest struct {
	Skills []string `json:"skills"`
}

// SkillsStore provides in-memory storage for skills and agent associations.
type SkillsStore struct {
	mu          sync.RWMutex
	skills      map[string]skills.Skill
	agentSkills map[string][]string // agent name -> skill names
}

// NewSkillsStore creates a new SkillsStore instance.
func NewSkillsStore() *SkillsStore {
	s := &SkillsStore{
		skills:      make(map[string]skills.Skill),
		agentSkills: make(map[string][]string),
	}

	// Pre-register default skills from the registry
	registry := skills.NewSkillsRegistry()
	for _, category := range []skills.SkillCategory{
		skills.Design,
		skills.Coding,
		skills.Testing,
		skills.Review,
		skills.Principles,
	} {
		for _, skill := range registry.GetByCategory(category) {
			s.skills[skill.Name] = skill
		}
	}

	return s
}

// RegisterSkill registers a new skill or updates an existing one.
func (s *SkillsStore) RegisterSkill(req SkillRequest) (*skills.Skill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("skills: name is required")
	}

	if req.Type == "" {
		return nil, fmt.Errorf("skills: type is required")
	}

	if req.Category == "" {
		return nil, fmt.Errorf("skills: category is required")
	}

	skill := skills.Skill{
		Name:           req.Name,
		Type:           skills.SkillType(req.Type),
		Category:       skills.SkillCategory(req.Category),
		Description:    req.Description,
		PromptTemplate: req.PromptTemplate,
		Parameters:     req.Parameters,
	}

	s.skills[skill.Name] = skill
	return &skill, nil
}

// GetSkill retrieves a skill by name.
func (s *SkillsStore) GetSkill(name string) (*skills.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	skill, exists := s.skills[name]
	if !exists {
		return nil, fmt.Errorf("skills: skill %q not found", name)
	}

	return &skill, nil
}

// DeleteSkill removes a skill from the store.
func (s *SkillsStore) DeleteSkill(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.skills[name]; !exists {
		return fmt.Errorf("skills: skill %q not found", name)
	}

	delete(s.skills, name)
	return nil
}

// ListSkills returns all skills grouped by category.
func (s *SkillsStore) ListSkills() map[string][]skills.Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()

	byCategory := make(map[string][]skills.Skill)
	for _, skill := range s.skills {
		category := string(skill.Category)
		byCategory[category] = append(byCategory[category], skill)
	}

	return byCategory
}

// SearchSkills searches skills by query string (name or description).
func (s *SkillsStore) SearchSkills(query string) []skills.Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if query == "" {
		var results []skills.Skill
		for _, skill := range s.skills {
			results = append(results, skill)
		}
		return results
	}

	query = strings.ToLower(query)
	var results []skills.Skill
	for _, skill := range s.skills {
		if strings.ToLower(skill.Name) == query ||
			strings.ToLower(skill.Description) == query {
			results = append(results, skill)
		}
	}

	return results
}

// ResetSkills restores all skills to their default state.
func (s *SkillsStore) ResetSkills() {
	s.mu.Lock()
	defer s.mu.Unlock()

	registry := skills.NewSkillsRegistry()
	for _, category := range []skills.SkillCategory{
		skills.Design,
		skills.Coding,
		skills.Testing,
		skills.Review,
		skills.Principles,
	} {
		for _, skill := range registry.GetByCategory(category) {
			s.skills[skill.Name] = skill
		}
	}
}

// ExportSkills exports all skills as a JSON-serializable map.
func (s *SkillsStore) ExportSkills() map[string]skills.Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()

	export := make(map[string]skills.Skill)
	for name, skill := range s.skills {
		export[name] = skill
	}

	return export
}

// ImportSkills imports skills from a JSON-serializable map.
func (s *SkillsStore) ImportSkills(imported map[string]skills.Skill) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for name, skill := range imported {
		if _, exists := s.skills[name]; !exists {
			s.skills[name] = skill
			count++
		}
	}

	return count
}

// GetAgentSkills returns the skills assigned to an agent.
func (s *SkillsStore) GetAgentSkills(agentName string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	skills, exists := s.agentSkills[agentName]
	if !exists {
		return nil, fmt.Errorf("agent %q not found", agentName)
	}

	return skills, nil
}

// SetAgentSkills assigns a list of skills to an agent.
func (s *SkillsStore) SetAgentSkills(agentName string, skillNames []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate that all skills exist
	for _, name := range skillNames {
		if _, exists := s.skills[name]; !exists {
			return fmt.Errorf("skills: skill %q not found", name)
		}
	}

	s.agentSkills[agentName] = skillNames
	return nil
}

// GetAvailableSkills returns all skills except those assigned to the agent.
func (s *SkillsStore) GetAvailableSkills(agentName string) []skills.Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()

	assigned := s.agentSkills[agentName]

	var available []skills.Skill
	for _, skill := range s.skills {
		isAssigned := false
		for _, name := range assigned {
			if name == skill.Name {
				isAssigned = true
				break
			}
		}
		if !isAssigned {
			available = append(available, skill)
		}
	}

	return available
}

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

// skillToResponse converts a skills.Skill to a SkillResponse.
func skillToResponse(s skills.Skill) SkillResponse {
	return SkillResponse{
		Name:           s.Name,
		Type:           string(s.Type),
		Category:       string(s.Category),
		Description:    s.Description,
		PromptTemplate: s.PromptTemplate,
		Parameters:     s.Parameters,
	}
}

// extractSkillID extracts the skill ID from the URL path.
// Expected format: /api/skills/{id} or /api/skills/{id}/...
func extractSkillID(path string) string {
	parts := SplitPath(path)
	if len(parts) < 4 {
		return ""
	}
	// parts: ["", "api", "skills", "{id}"]
	if parts[2] != "skills" {
		return ""
	}
	return parts[3]
}

// extractAgentID extracts the agent ID from the URL path.
// Expected format: /api/agents/{id}/...
func extractAgentID(path string) string {
	parts := SplitPath(path)
	if len(parts) < 4 {
		return ""
	}
	// parts: ["", "api", "agents", "{id}"]
	if parts[2] != "agents" {
		return ""
	}
	return parts[3]
}
