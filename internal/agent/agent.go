package agent

import (
	"context"
)

// Agent defines the interface that all agents in the multi-agent system must implement.
// This abstraction enables polymorphism and dependency inversion,
// allowing different agent types (planning, coding, testing, etc.) to be swapped
// without modifying the orchestrator or other dependents.
type Agent interface {
	// Name returns the agent's identifier.
	Name() string

	// Execute runs the agent with the given context and input, returning the result.
	Execute(ctx context.Context, input string) (*AgentResult, error)

	// GetSkills returns the list of skills assigned to this agent.
	GetSkills() []string

	// SetSkills assigns a list of skills to this agent.
	SetSkills(skills []string)
}

// AgentResult holds the output and metadata produced by an agent's execution.
type AgentResult struct {
	// Output contains the primary result of the agent's execution.
	Output string

	// Metadata holds additional key-value information from the execution.
	Metadata map[string]string

	// Phase indicates the phase this agent was executed in.
	Phase string
}
