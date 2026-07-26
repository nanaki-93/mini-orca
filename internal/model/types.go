package model

// ProviderConfig represents an LLM provider configuration (e.g., LM Studio, Ollama, OpenAI)
type ProviderConfig struct {
	Name            string   `json:"name"`
	BaseURL         string   `json:"base_url"`
	AvailableModels []string `json:"available_models,omitempty"`
	APIKey          string   `json:"api_key,omitempty"`
}

// Model represents an available model from a provider
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// ChatMessage represents a single message in a chat conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request payload for a chat completion call
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float32       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatChoice represents a single choice in the response
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// ChatUsage tracks token usage for billing/monitoring
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse is the response from a chat completion call
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage"`
}

// ModelConfig holds the configuration for a single model invocation
type ModelConfig struct {
	Provider    string  `json:"provider"`
	ModelID     string  `json:"model_id"`
	Temperature float32 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// Phase represents a phase in the orchestrator flow
type Phase string

const (
	PhasePlanning    Phase = "planning"
	PhaseCoding      Phase = "coding"
	PhaseTesting     Phase = "testing"
	PhaseReview      Phase = "review"
	PhaseHumanReview Phase = "human_review"
)

// PhaseConfig holds model configuration specific to a phase
type PhaseConfig struct {
	ModelConfig
	Temperature float32 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// AgentConfig holds configuration for an agent in the orchestrator
type AgentConfig struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Phase       Phase  `json:"phase"`
	ModelConfig `json:"model_config"`
}

// Config is the top-level configuration for the model subsystem
type Config struct {
	ActiveProvider string                    `json:"active_provider"`
	Providers      map[string]ProviderConfig `json:"providers"`
	Phases         map[Phase]PhaseConfig     `json:"phases"`
	Agents         map[string]AgentConfig    `json:"agents"`
}

// ProviderNotFoundError is returned when a provider is not found
type ProviderNotFoundError struct {
	ProviderName string
}

func (e *ProviderNotFoundError) Error() string {
	return "provider not found: " + e.ProviderName
}
