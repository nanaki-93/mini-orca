package skills

import (
	"fmt"
	"sync"

	"mini-orca/internal/types"
)

// Registry manages custom skills registration and lookup.
type Registry struct {
	customSkills map[string]types.Skill
	mu           sync.RWMutex
}

// NewRegistry creates a new skills registry.
func NewRegistry() *Registry {
	return &Registry{
		customSkills: make(map[string]types.Skill),
	}
}

// Register adds a custom skill to the registry.
func (r *Registry) Register(skill types.Skill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.customSkills[skill.Name] = skill
	return nil
}

// Get retrieves a custom skill by name.
func (r *Registry) Get(name string) (types.Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.customSkills[name]
	if !ok {
		return types.Skill{}, fmt.Errorf("custom skill not found: %s", name)
	}

	return skill, nil
}

// List returns all custom skills.
func (r *Registry) List() []types.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skills := make([]types.Skill, 0, len(r.customSkills))
	for _, s := range r.customSkills {
		skills = append(skills, s)
	}
	return skills
}

// Delete removes a custom skill by name.
func (r *Registry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.customSkills[name]; !ok {
		return fmt.Errorf("custom skill not found: %s", name)
	}

	delete(r.customSkills, name)
	return nil
}

// MergeWithLibrary combines custom skills with the predefined library.
// Custom skills with the same name as library skills will override them.
func (r *Registry) MergeWithLibrary() []types.Skill {
	library := Library()
	merged := make(map[string]types.Skill)

	// Start with library skills
	for _, s := range library {
		merged[s.Name] = s
	}

	// Overlay custom skills
	r.mu.RLock()
	for _, s := range r.customSkills {
		merged[s.Name] = s
	}
	r.mu.RUnlock()

	result := make([]types.Skill, 0, len(merged))
	for _, s := range merged {
		result = append(result, s)
	}

	return result
}
