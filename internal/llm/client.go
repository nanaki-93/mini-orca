package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/logging"
)

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

// Model represents an available model from a provider
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// Client handles all LLM API interactions with a single configuration
type Client struct {
	baseURL     string
	apiKey      string
	model       string
	temperature float32
	maxTokens   int
	httpClient  *http.Client
}

// NewClient creates a new LLM client with the given configuration
func NewClient(baseURL, apiKey, model string, temperature float32, maxTokens int) *Client {
	return &Client{
		baseURL:     baseURL,
		apiKey:      apiKey,
		model:       model,
		temperature: temperature,
		maxTokens:   maxTokens,
		httpClient:  &http.Client{Timeout: 5 * time.Minute},
	}
}

// Chat sends a chat completion request and returns the response
func (c *Client) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("llm client: base URL not configured")
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("llm client: messages are required")
	}

	req := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
	}

	body, err := json.Marshal(req)
	if err != nil {
		logging.Error("Failed to marshal request", "error", err)
		return nil, fmt.Errorf("llm client: failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat/completions", c.baseURL)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		logging.Error("Failed to create request", "error", err)
		return nil, fmt.Errorf("llm client: failed to create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(request)
	if err != nil {
		logging.Error("Request failed", "error", err)
		return nil, fmt.Errorf("llm client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logging.Error("Unexpected status code", "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("llm client: unexpected status code: %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		logging.Error("Failed to decode response", "error", err)
		return nil, fmt.Errorf("llm client: failed to decode response: %w", err)
	}

	logging.Info("Chat completed", "model", chatResp.Model, "tokens", chatResp.Usage.TotalTokens)
	return &chatResp, nil
}

// ListModels returns the list of available models from the LLM provider
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("llm client: base URL not configured")
	}

	url := fmt.Sprintf("%s/v1/models", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logging.Error("Failed to create request", "error", err)
		return nil, fmt.Errorf("llm client: failed to create request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.Error("Request failed", "error", err)
		return nil, fmt.Errorf("llm client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logging.Error("Unexpected status code", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("llm client: unexpected status code: %d", resp.StatusCode)
	}

	var listResp struct {
		Object string       `json:"object"`
		Data   []modelEntry `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		logging.Error("Failed to decode response", "error", err)
		return nil, fmt.Errorf("llm client: failed to decode response: %w", err)
	}

	models := make([]Model, 0, len(listResp.Data))
	for _, entry := range listResp.Data {
		models = append(models, Model{
			ID:      entry.ID,
			Object:  entry.Object,
			OwnedBy: entry.OwnedBy,
		})
	}

	logging.Info("Listed models", "count", len(models))
	return models, nil
}

// modelEntry represents a single model entry in the /v1/models response
type modelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}
