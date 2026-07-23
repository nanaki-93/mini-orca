package agent

import (
	"context"
	"fmt"
	"strings"

	"mini-orca/internal/types"
)

// Agent is the interface that all specialized agents must implement.
type Agent interface {
	// Name returns the agent's name.
	Name() string

	// Skills returns the list of skills this agent has.
	Skills() []types.Skill

	// Execute runs the agent with the given context and returns the result.
	Execute(ctx context.Context, input string) (*ExecutionResult, error)
}

// ExecutionResult represents the output from an agent execution.
type ExecutionResult struct {
	Output     string
	Phase      string
	SkillsUsed []string
	Success    bool
	Error      string
}

// GetSkillPrompt returns a formatted prompt section for a list of skills.
func GetSkillPrompt(skills []types.Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## Your Skills\n")
	sb.WriteString("You have the following skills available:\n\n")

	for _, s := range skills {
		sb.WriteString(fmt.Sprintf("### %s (%s)\n", s.Name, s.Type))
		sb.WriteString(fmt.Sprintf("%s\n", s.Description))
		if s.Prompt != "" {
			sb.WriteString(fmt.Sprintf("**Guidance:** %s\n", s.Prompt))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
