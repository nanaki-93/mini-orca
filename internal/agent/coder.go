package agent

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/agent/skills"
	"github.com/nanaki-93/mini-orca/v2/internal/model"
)

// CoderAgent is the agent responsible for implementing code.
// It embeds *Client to inherit Agent interface implementation.
type CoderAgent struct {
	*Client
	registry *skills.SkillsRegistry
}

// NewCoderAgent creates a new coder agent with the given router and skills registry.
func NewCoderAgent(router *model.Router, registry *skills.SkillsRegistry) *CoderAgent {
	client := NewClient(router)
	client.name = "coder"
	client.description = "Implements code based on plans and specifications"
	client.phase = model.PhaseCoding
	client.skills = make([]string, 0)

	return &CoderAgent{
		Client:   client,
		registry: registry,
	}
}

// Execute runs the coder agent with the given atomic unit description.
// It builds a system prompt from coder skills, calls the LLM, and returns the code.
// Each execution handles exactly ONE atomic unit (function, struct, or class).
func (c *CoderAgent) Execute(ctx context.Context, input string) (*Result, error) {
	if c.router == nil {
		return nil, fmt.Errorf("coder agent: router not configured")
	}

	if input == "" {
		return nil, fmt.Errorf("coder agent: input is required")
	}

	if c.registry == nil {
		return nil, fmt.Errorf("coder agent: skills registry not configured")
	}

	// Build system prompt from coder skills
	systemPrompt := c.registry.BuildSkillPrompt(c.GetSkills())
	if systemPrompt == "" {
		systemPrompt = "You are an expert Go coder. Generate clean, idiomatic, well-tested code."
	}

	// Combine system prompt with user input (atomic unit description)
	fullPrompt := systemPrompt + "\n\n---\n\nAtomic Unit Description:\n" + input

	// Call the LLM with coding phase config
	messages := []model.ChatMessage{
		{Role: "user", Content: fullPrompt},
	}

	resp, err := c.router.Chat(string(c.phase), messages)
	if err != nil {
		return nil, fmt.Errorf("coder agent: execution failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("coder agent: empty response from model")
	}

	return &Result{
		Output: resp.Choices[0].Message.Content,
		Metadata: map[string]string{
			"model":  resp.Model,
			"phase":  string(c.phase),
			"tokens": fmt.Sprintf("%d", resp.Usage.TotalTokens),
		},
		Phase: string(c.phase),
	}, nil
}
