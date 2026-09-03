package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/logging"
)

const (
	maxProviderResponseBytes = 4 * 1024 * 1024
	providerRequestTimeout   = 5 * time.Minute
)

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the OpenAI-compatible chat-completions request payload.
type ChatRequest struct {
	Model           string        `json:"model"`
	Messages        []ChatMessage `json:"messages"`
	Temperature     float32       `json:"temperature,omitempty"`
	MaxTokens       int           `json:"max_tokens,omitempty"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
	Stream          bool          `json:"stream,omitempty"`
}

// ChatChoice represents a single choice in a response.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// ChatUsage tracks token usage for billing and monitoring.
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse is the response from a chat completion call.
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage"`
}

// Client owns the one provider-neutral Chat Completions request flow.
type Client struct {
	profile    config.ModelProfile
	httpClient *http.Client
}

// NewClient constructs the one LLM client from a validated fixed scope.
func NewClient(profile config.ModelProfile) *Client {
	return &Client{
		profile:    profile,
		httpClient: &http.Client{Timeout: providerRequestTimeout},
	}
}

// Chat sends one OpenAI-compatible Chat Completions request and validates the
// provider result before application code receives it.
func (c *Client) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("llm client: messages are required")
	}
	requestBody, err := json.Marshal(ChatRequest{
		Model:           c.profile.Model,
		Messages:        messages,
		Temperature:     c.profile.Temperature,
		MaxTokens:       c.profile.MaxTokens,
		ReasoningEffort: c.profile.ReasoningEffort,
	})
	if err != nil {
		return nil, fmt.Errorf("llm client: marshal chat request: %w", err)
	}

	request, err := c.newRequest(ctx, http.MethodPost, "chat/completions", bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	responseBody, err := c.do(request)
	if err != nil {
		return nil, err
	}

	var response ChatResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("llm client: decode chat response: %w", err)
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("llm client: chat response has no content")
	}
	logging.Info("Chat completed", "model", response.Model, "tokens", response.Usage.TotalTokens)
	return &response, nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	endpoint, err := joinAPIURL(c.profile.APIBaseURL, path)
	if err != nil {
		return nil, fmt.Errorf("llm client: build request URL: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("llm client: create request: %w", err)
	}
	if c.profile.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.profile.APIKey)
	}
	return request, nil
}

func (c *Client) do(request *http.Request) ([]byte, error) {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("llm client: request failed: %w", err)
	}
	defer response.Body.Close()

	data, err := readProviderBody(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, providerStatusError(response.StatusCode, data)
	}
	return data, nil
}

func joinAPIURL(apiBaseURL, path string) (string, error) {
	base, err := url.Parse(apiBaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("API base URL is not configured")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(path, "/")
	return base.String(), nil
}

func providerStatusError(status int, body []byte) error {
	var payload struct {
		Error json.RawMessage `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)
	return fmt.Errorf("llm client: provider returned status %d", status)
}

func readProviderBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxProviderResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("llm client: read response: %w", err)
	}
	if len(data) > maxProviderResponseBytes {
		return nil, fmt.Errorf("llm client: response exceeds %d byte limit", maxProviderResponseBytes)
	}
	return data, nil
}
