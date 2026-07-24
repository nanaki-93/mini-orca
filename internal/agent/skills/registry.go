package skills

import (
	"fmt"
	"sync"

	"mini-orca/internal/types"
)

// Skill wraps types.Skill with additional metadata.
type Skill struct {
	types.Skill
	ID        string `json:"id"`
	Enabled   bool   `json:"enabled"`
	AgentKeys []string `json:"agent_keys,omitempty"`
}

// Registry manages custom skills registration and lookup.
type Registry struct {
	customSkills map[string]Skill
	agentSkills  map[string][]string // agent -> skill IDs
	mu           sync.RWMutex
}

// NewRegistry creates a new skills registry.
func NewRegistry() *Registry {
	return &Registry{
		customSkills: make(map[string]Skill),
		agentSkills:  make(map[string][]string),
	}
}

// NewSkill creates a new skill with a generated ID.
func NewSkill(name, description, skillType string) *Skill {
	return &Skill{
		Skill: types.Skill{
			Name:        name,
			Description: description,
			Type:        types.SkillType(skillType),
			Priority:    3,
		},
		Enabled: true,
	}
}

// Add adds a skill to the registry.
func (r *Registry) Add(skill *Skill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.customSkills[skill.Name] = *skill
	return nil
}

// Update updates an existing skill in the registry.
func (r *Registry) Update(skill *Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.customSkills[skill.Name]; !ok {
		return fmt.Errorf("skill not found: %s", skill.Name)
	}

	r.customSkills[skill.Name] = *skill
	return nil
}

// Remove removes a skill from the registry.
func (r *Registry) Remove(skillName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.customSkills[skillName]; !ok {
		return fmt.Errorf("skill not found: %s", skillName)
	}

	delete(r.customSkills, skillName)
	return nil
}

// Get retrieves a skill by name.
func (r *Registry) Get(name string) (*Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.customSkills[name]
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", name)
	}

	return &skill, nil
}

// List returns all skills.
func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skills := make([]*Skill, 0, len(r.customSkills))
	for _, s := range r.customSkills {
		skills = append(skills, &s)
	}
	return skills
}

// IsAgentSkill checks if a skill is assigned to an agent.
func (r *Registry) IsAgentSkill(agent, skillID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, id := range r.agentSkills[agent] {
		if id == skillID {
			return true
		}
	}
	return false
}

// AssignAgentSkill assigns a skill to an agent.
func (r *Registry) AssignAgentSkill(agent, skillID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if skill exists
	if _, ok := r.customSkills[skillID]; !ok {
		return fmt.Errorf("skill not found: %s", skillID)
	}

	// Add to agent's skills if not already present
	for _, id := range r.agentSkills[agent] {
		if id == skillID {
			return nil // Already assigned
		}
	}

	r.agentSkills[agent] = append(r.agentSkills[agent], skillID)
	return nil
}

// GetAgentSkills returns all skills assigned to an agent.
func (r *Registry) GetAgentSkills(agent string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.agentSkills[agent]
}

// GetSkillsByAgent returns skills for a specific agent.
func (r *Registry) GetSkillsByAgent(agent string) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skillIDs := r.agentSkills[agent]
	skills := make([]*Skill, 0, len(skillIDs))

	for _, id := range skillIDs {
		if skill, ok := r.customSkills[id]; ok {
			skills = append(skills, &skill)
		}
	}

	return skills
}

// MergeWithLibrary combines custom skills with the predefined library.
// Custom skills with the same name as library skills will override them.
func (r *Registry) MergeWithLibrary() []*Skill {
	library := Library()
	merged := make(map[string]*Skill)

	// Start with library skills
	for _, s := range library {
		merged[s.Name] = &Skill{
			Skill:   s.Skill,
			Enabled: true,
		}
	}

	// Overlay custom skills
	r.mu.RLock()
	for _, s := range r.customSkills {
		merged[s.Name] = &s
	}
	r.mu.RUnlock()

	result := make([]*Skill, 0, len(merged))
	for _, s := range merged {
		result = append(result, s)
	}

	return result
}
