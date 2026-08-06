package api

import (
	"fmt"
	"strings"
	"sync"

	"github.com/nanaki-93/mini-orca/v2/internal/agent/skills"
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
