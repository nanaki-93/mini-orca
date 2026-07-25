package model

import (
	"context"
)

// Provider defines the interface that all LLM providers must implement.
// This abstraction allows switching between providers (LM Studio, Ollama, OpenAI, etc.)
// without changing the rest of the codebase.
type Provider interface {
	// Chat sends a chat completion request to the provider and returns the response.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// ListModels returns the list of available models from the provider.
	ListModels(ctx context.Context) ([]Model, error)

	// Name returns the name of the provider (e.g., "lm-studio", "ollama", "openai").
	Name() string

	// IsStreamingSupported returns whether the provider supports streaming responses.
	// This is used to determine whether to enable streaming UI features.
	IsStreamingSupported() bool
}
