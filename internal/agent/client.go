package agent

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

// Client handles agent interactions with LLM providers.
// It implements the Agent interface for the multi-agent system.
type Client struct {
	llmClient   *llm.Client
	name        string
	description string
	phase       string // "coding", "testing", "review"
	skills      []string
}

// NewClient creates a new agent client with the given LLM client.
func NewClient(llmClient *llm.Client) *Client {
	return &Client{
		llmClient: llmClient,
		skills:    make([]string, 0),
	}
}

// Name returns the agent's identifier.
func (c *Client) Name() string {
	return c.name
}

// Description returns a human-readable description of the agent's purpose.
func (c *Client) Description() string {
	return c.description
}

// Phase returns the phase this agent is responsible for.
func (c *Client) Phase() string {
	return c.phase
}

// Execute runs the agent with the given context and input, returning the result.
// It implements the Agent interface.
func (c *Client) Execute(ctx context.Context, input string) (*Result, error) {
	if c.llmClient == nil {
		return nil, fmt.Errorf("agent client: LLM client not configured")
	}

	if input == "" {
		return nil, fmt.Errorf("agent client: input is required")
	}

	messages := []llm.ChatMessage{{Role: "user", Content: input}}

	resp, err := c.llmClient.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("agent client: execution failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("agent client: empty response from model")
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

// GetSkills returns the list of skills assigned to this agent.
// It implements the Agent interface.
func (c *Client) GetSkills() []string {
	if c.skills == nil {
		return []string{}
	}
	return c.skills
}

// SetSkills assigns a list of skills to this agent.
// It implements the Agent interface.
func (c *Client) SetSkills(skills []string) {
	c.skills = skills
}

// ListModels returns the list of available models from the LLM provider.
func (c *Client) ListModels() ([]llm.Model, error) {
	if c.llmClient == nil {
		return nil, fmt.Errorf("agent client: LLM client not configured")
	}

	return c.llmClient.ListModels(context.Background())
}
