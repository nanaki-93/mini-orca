package agent

import (
	"mini-orca/internal/agent/skills"
	"mini-orca/internal/model"
	"mini-orca/internal/types"
)

// skillNames returns the names of the given skills.
func skillNames(skills []types.Skill) []string {
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	return names
}

// InitializeAgentRegistry creates and registers all agents with their skills.
func InitializeAgentRegistry(r *Registry, agentSkills map[string][]string) {
	// Register each agent type with its skills
	for name, skillNames := range agentSkills {
		agentSkills := skills.GetSkillsByName(skillNames)
		
		switch name {
		case "planner":
			r.Register(NewPlannerAgent(agentSkills, nil))
		case "coder":
			r.Register(NewCoderAgent(agentSkills, nil))
		case "tester":
			r.Register(NewTesterAgent(agentSkills, nil))
		case "reviewer":
			r.Register(NewReviewerAgent(agentSkills, nil))
		}
	}
}

// InitializeAgentsWithRouter creates agents with a model router and registers them.
func InitializeAgentsWithRouter(r *Registry, config map[string][]string, router *model.Router) {
	for name, skillNames := range config {
		agentSkills := skills.GetSkillsByName(skillNames)
		
		switch name {
		case "planner":
			r.Register(NewPlannerAgent(agentSkills, router))
		case "coder":
			r.Register(NewCoderAgent(agentSkills, router))
		case "tester":
			r.Register(NewTesterAgent(agentSkills, router))
		case "reviewer":
			r.Register(NewReviewerAgent(agentSkills, router))
		}
	}
}
