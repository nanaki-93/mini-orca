package agent

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/internal/model"
)

// Client handles agent interactions with LLM providers through the model router.
// It implements the Agent interface for the multi-agent system.
type Client struct {
	router      *model.Router
	name        string
	description string
	phase       model.Phase
	skills      []string
}

// NewClient creates a new agent client with the given router.
func NewClient(router *model.Router) *Client {
	return &Client{
		router: router,
		skills: make([]string, 0),
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
func (c *Client) Phase() model.Phase {
	return c.phase
}

// Execute runs the agent with the given context and input, returning the result.
// It implements the Agent interface.
func (c *Client) Execute(ctx context.Context, input string) (*AgentResult, error) {
	if c.router == nil {
		return nil, fmt.Errorf("agent client: router not configured")
	}

	if input == "" {
		return nil, fmt.Errorf("agent client: input is required")
	}

	messages := []model.ChatMessage{{Role: "user", Content: input}}

	resp, err := c.router.Chat(string(c.phase), messages)
	if err != nil {
		return nil, fmt.Errorf("agent client: execution failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("agent client: empty response from model")
	}

	return &AgentResult{
		Output: resp.Choices[0].Message.Content,
		Metadata: map[string]string{
			"model":  resp.Model,
			"phase":  string(c.phase),
			"tokens": fmt.Sprintf("%d", resp.Usage.TotalTokens),
		},
		Phase: string(c.phase),
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

// ListModels returns the list of available models from the active provider.
func (c *Client) ListModels() ([]model.Model, error) {
	if c.router == nil {
		return nil, fmt.Errorf("agent client: router not configured")
	}

	return c.router.RouteListModels(context.Background())
}
