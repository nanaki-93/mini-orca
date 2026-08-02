package skills

import (
	"fmt"
	"sync"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/logging"
)

// SkillsRegistry manages the lifecycle and lookup of skills in the multi-agent system.
// It provides thread-safe registration and retrieval of skills by name and category.
type SkillsRegistry struct {
	mu         sync.RWMutex
	skills     map[string]Skill          // keyed by skill name
	byCategory map[SkillCategory][]Skill // keyed by category
}

// NewSkillsRegistry creates a new skills registry and pre-registers all skills from the library.
func NewSkillsRegistry() *SkillsRegistry {
	r := &SkillsRegistry{
		skills:     make(map[string]Skill),
		byCategory: make(map[SkillCategory][]Skill),
	}

	// Pre-register all skills from the library
	for _, skill := range AllSkills() {
		_ = r.Register(skill)
	}

	return r
}

// Register adds a skill to the registry.
// It returns an error if a skill with the same name is already registered.
func (r *SkillsRegistry) Register(skill Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[skill.Name]; exists {
		return fmt.Errorf("skills: skill %q already registered", skill.Name)
	}

	r.skills[skill.Name] = skill
	r.byCategory[skill.Category] = append(r.byCategory[skill.Category], skill)

	return nil
}

// Get retrieves a skill by name.
// It returns an error if the skill is not found.
func (r *SkillsRegistry) Get(name string) (*Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, exists := r.skills[name]
	if !exists {
		return nil, fmt.Errorf("skills: skill %q not found", name)
	}

	return &skill, nil
}

// GetByCategory returns all skills in the given category.
func (r *SkillsRegistry) GetByCategory(category SkillCategory) []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.byCategory[category]
}

// GetForAgent returns all skills associated with the given agent.
// It uses the agent name to determine the appropriate category.
func (r *SkillsRegistry) GetForAgent(agentName string) []Skill {
	category := agentToCategory(agentName)
	return r.GetByCategory(category)
}

// agentToCategory maps an agent name to its primary skill category.
func agentToCategory(agentName string) SkillCategory {
	switch agentName {
	case "coder":
		return Coding
	case "tester":
		return Testing
	case "reviewer":
		return Review
	default:
		return Principles
	}
}

// InitSkillsRegistry creates a skills registry and registers all skills from the config.
func InitSkillsRegistry(cfg *config.Config) *SkillsRegistry {
	registry := NewSkillsRegistry()

	// Register knowledge skills from config
	for name, description := range cfg.Skills.Knowledge {
		s := Skill{
			Name:           name,
			Type:           Knowledge,
			PromptTemplate: description,
		}
		if err := registry.Register(s); err != nil {
			logging.Warn("Failed to register knowledge skill", "name", name, "error", err)
		}
	}

	// Register tool skills from config
	for name, description := range cfg.Skills.Tools {
		s := Skill{
			Name:           name,
			Type:           Tool,
			PromptTemplate: description,
		}
		if err := registry.Register(s); err != nil {
			logging.Warn("Failed to register tool skill", "name", name, "error", err)
		}
	}

	totalSkills := len(cfg.Skills.Knowledge) + len(cfg.Skills.Tools)
	logging.Info("Skills registry initialized", "count", totalSkills)

	return registry
}
