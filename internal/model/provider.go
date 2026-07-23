package model

import "context"

// Message represents a single message in a chat conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a request to the chat completion API.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
}

// ChatResponse represents a response from the chat completion API.
type ChatResponse struct {
	Choices []Choice `json:"choices"`
	Model   string   `json:"model"`
}

// Choice represents a single choice in a chat response.
type Choice struct {
	Message    Message `json:"message"`
	Index      int     `json:"index"`
	FinishReason string `json:"finish_reason"`
}

// ModelConfig holds the configuration for a specific model.
type ModelConfig struct {
	Provider    string   `json:"provider"`
	Model       string   `json:"model"`
	BaseURL     string   `json:"base_url,omitempty"`
	APIKey      string   `json:"api_key,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	TopP        float64  `json:"top_p,omitempty"`
	Timeout     int      `json:"timeout,omitempty"` // seconds
}

// Provider is the interface that all LLM providers must implement.
type Provider interface {
	// ListModels returns the available models from the provider.
	ListModels(ctx context.Context) ([]string, error)

	// Chat sends a chat request and returns the response.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// Name returns the provider's name identifier.
	Name() string
}
