package agent

import (
	"mini-orca/internal/agent/skills"
	"mini-orca/internal/model"
	"mini-orca/internal/types"
)

// skillNames returns the names of the given skills.
func skillNames(skills []*skills.Skill) []string {
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	return names
}

// skillsToTypes converts []*skills.Skill to []types.Skill.
func skillsToTypes(skillList []*skills.Skill) []types.Skill {
	result := make([]types.Skill, len(skillList))
	for i, s := range skillList {
		result[i] = s.Skill
	}
	return result
}

// typesToSkills converts []types.Skill to []*skills.Skill.
func typesToSkills(skillList []types.Skill) []*skills.Skill {
	result := make([]*skills.Skill, len(skillList))
	for i, s := range skillList {
		result[i] = &skills.Skill{
			Skill:   s,
			Enabled: true,
		}
	}
	return result
}

// InitializeAgentRegistry creates and registers all agents with their skills.
func InitializeAgentRegistry(r *Registry, agentSkills map[string][]string) {
	// Register each agent type with its skills
	for name, skillNames := range agentSkills {
		agentSkills := skills.GetSkillsByName(skillNames)
		
		switch name {
		case "planner":
			r.Register(NewPlannerAgent(skillsToTypes(agentSkills), nil))
		case "coder":
			r.Register(NewCoderAgent(skillsToTypes(agentSkills), nil))
		case "tester":
			r.Register(NewTesterAgent(skillsToTypes(agentSkills), nil))
		case "reviewer":
			r.Register(NewReviewerAgent(skillsToTypes(agentSkills), nil))
		}
	}
}

// InitializeAgentsWithRouter creates agents with a model router and registers them.
func InitializeAgentsWithRouter(r *Registry, config map[string][]string, router *model.Router) {
	for name, skillNames := range config {
		agentSkills := skills.GetSkillsByName(skillNames)
		
		switch name {
		case "planner":
			r.Register(NewPlannerAgent(skillsToTypes(agentSkills), router))
		case "coder":
			r.Register(NewCoderAgent(skillsToTypes(agentSkills), router))
		case "tester":
			r.Register(NewTesterAgent(skillsToTypes(agentSkills), router))
		case "reviewer":
			r.Register(NewReviewerAgent(skillsToTypes(agentSkills), router))
		}
	}
}
