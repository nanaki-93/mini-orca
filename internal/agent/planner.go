package agent

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/internal/agent/skills"
	"github.com/nanaki-93/mini-orca/internal/model"
)

// PlannerAgent is the agent responsible for planning tasks.
// It embeds *Client to inherit Agent interface implementation.
type PlannerAgent struct {
	*Client
	registry *skills.SkillsRegistry
}

// NewPlannerAgent creates a new planner agent with the given router and skills registry.
func NewPlannerAgent(router *model.Router, registry *skills.SkillsRegistry) *PlannerAgent {
	client := NewClient(router)
	client.name = "planner"
	client.description = "Plans and breaks down tasks into actionable subtasks"
	client.phase = model.PhasePlanning
	client.skills = make([]string, 0)

	return &PlannerAgent{
		Client:   client,
		registry: registry,
	}
}

// Execute runs the planner agent with the given input (goal description).
// It builds a system prompt from planner skills, calls the LLM, and returns the plan.
func (p *PlannerAgent) Execute(ctx context.Context, input string) (*AgentResult, error) {
	if p.router == nil {
		return nil, fmt.Errorf("planner agent: router not configured")
	}

	if input == "" {
		return nil, fmt.Errorf("planner agent: input is required")
	}

	if p.registry == nil {
		return nil, fmt.Errorf("planner agent: skills registry not configured")
	}

	// Build system prompt from planner skills
	systemPrompt := p.registry.BuildSkillPrompt(p.GetSkills())
	if systemPrompt == "" {
		systemPrompt = "You are a task planning agent. Break down the given goal into clear, actionable subtasks."
	}

	// Combine system prompt with user input
	fullPrompt := systemPrompt + "\n\n---\n\nGoal: " + input

	// Call the LLM with planning phase config
	messages := []model.ChatMessage{
		{Role: "user", Content: fullPrompt},
	}

	resp, err := p.router.Chat(string(p.phase), messages)
	if err != nil {
		return nil, fmt.Errorf("planner agent: execution failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("planner agent: empty response from model")
	}

	return &AgentResult{
		Output: resp.Choices[0].Message.Content,
		Metadata: map[string]string{
			"model":  resp.Model,
			"phase":  string(p.phase),
			"tokens": fmt.Sprintf("%d", resp.Usage.TotalTokens),
		},
		Phase: string(p.phase),
	}, nil
}
