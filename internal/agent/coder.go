package agent

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

// CoderAgent is the agent responsible for implementing code.
// It embeds *Client to inherit Agent interface implementation.
type CoderAgent struct {
	*Client
}

// NewCoderAgent creates a new coder agent with the given LLM client.
func NewCoderAgent(llmClient *llm.Client) *CoderAgent {
	client := NewClient(llmClient)
	client.name = "coder"
	client.description = "Implements code based on plans and specifications"
	client.phase = "coding"
	client.skills = make([]string, 0)

	return &CoderAgent{
		Client: client,
	}
}

// Execute runs the coder agent with the given atomic unit description.
// It builds a system prompt from the agent's skills, calls the LLM, and returns the code.
// Each execution handles exactly ONE atomic unit (function, struct, or class).
func (c *CoderAgent) Execute(ctx context.Context, input string) (*Result, error) {
	if c.llmClient == nil {
		return nil, fmt.Errorf("coder agent: LLM client not configured")
	}

	if input == "" {
		return nil, fmt.Errorf("coder agent: input is required")
	}

	// Build system prompt from coder skills
	systemPrompt := c.buildSkillPrompt(c.GetSkills())
	if systemPrompt == "" {
		systemPrompt = "You are an expert Go coder. Generate clean, idiomatic, well-tested code."
	}

	// Combine system prompt with user input (atomic unit description)
	fullPrompt := systemPrompt + "\n\n---\n\nAtomic Unit Description:\n" + input

	// Call the LLM
	messages := []llm.ChatMessage{
		{Role: "user", Content: fullPrompt},
	}

	resp, err := c.llmClient.Chat(ctx, messages)
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
			"phase":  c.phase,
			"tokens": fmt.Sprintf("%d", resp.Usage.TotalTokens),
		},
		Phase: c.phase,
	}, nil
}

// buildSkillPrompt constructs a system prompt from the given skill names.
func (c *CoderAgent) buildSkillPrompt(skillNames []string) string {
	if len(skillNames) == 0 {
		return ""
	}

	var prompt string
	for _, skillName := range skillNames {
		prompt += "Use the " + skillName + " skill.\n"
	}
	return prompt
}
