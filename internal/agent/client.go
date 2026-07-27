package agent

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/internal/model"
)

// Client handles agent interactions with LLM providers through the model router.
type Client struct {
	router *model.Router
}

// NewClient creates a new agent client with the given router.
func NewClient(router *model.Router) *Client {
	return &Client{router: router}
}

// Generate sends a chat request to the LLM using the specified phase configuration.
func (c *Client) Generate(phase string, messages []model.ChatMessage) (*model.ChatResponse, error) {
	if c.router == nil {
		return nil, fmt.Errorf("agent client: router not configured")
	}

	if phase == "" {
		return nil, fmt.Errorf("agent client: phase is required")
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("agent client: at least one message is required")
	}

	return c.router.Chat(phase, messages)
}

// ListModels returns the list of available models from the active provider.
func (c *Client) ListModels() ([]model.Model, error) {
	if c.router == nil {
		return nil, fmt.Errorf("agent client: router not configured")
	}

	return c.router.RouteListModels(context.Background())
}
