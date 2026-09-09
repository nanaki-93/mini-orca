package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
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

// ErrRedirectRejected reports that a provider attempted to redirect a
// prompt-bearing request. Redirects are deliberately not followed: a redirect
// can change the destination that receives source context and credentials.
var ErrRedirectRejected = errors.New("provider redirect rejected")

// ErrRequestRejected reports a provider-side request validation failure. It is
// intentionally distinct from transient transport and server failures so a
// caller cannot turn an unsupported request format into repeated attempts.
var ErrRequestRejected = errors.New("provider rejected request")

// ErrStructuredRequestRejected reports a non-retryable 400 response to a
// request that included a strict schema. The response does not identify which
// request field the provider rejected, so callers must not infer that it was
// specifically the response format.
var ErrStructuredRequestRejected = errors.New("provider rejected structured request")

// ErrUnusableResponse reports a provider response that an ordinary application
// caller cannot use as final assistant output.
var ErrUnusableResponse = errors.New("provider response is unusable")

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	Reasoning        string `json:"reasoning,omitempty"`
}

// ChatRequest is the OpenAI-compatible chat-completions request payload.
type ChatRequest struct {
	Model           string          `json:"model"`
	Messages        []ChatMessage   `json:"messages"`
	Temperature     float32         `json:"temperature"`
	MaxTokens       int             `json:"max_tokens,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	TopP            *float32        `json:"top_p,omitempty"`
	TopK            *int            `json:"top_k,omitempty"`
	MinP            *float32        `json:"min_p,omitempty"`
	PresencePenalty *float32        `json:"presence_penalty,omitempty"`
	RepeatPenalty   *float32        `json:"repeat_penalty,omitempty"`
	ResponseFormat  *ResponseFormat `json:"response_format,omitempty"`
	Stream          bool            `json:"stream,omitempty"`
}

// JSONSchema defines one strict structured-output contract for a chat request.
// Schema is raw JSON because JSON Schema has open-ended keywords, while this
// boundary still validates that callers provide one object before transport.
type JSONSchema struct {
	Name   string
	Schema json.RawMessage
}

// Identity binds a resumable caller to the exact strict schema it sent.
func (schema JSONSchema) Identity() (string, error) {
	format, err := schema.responseFormat()
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(format)
	if err != nil {
		return "", fmt.Errorf("llm client: marshal response format identity: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", sum[:]), nil
}

func (schema JSONSchema) responseFormat() (*ResponseFormat, error) {
	if strings.TrimSpace(schema.Name) == "" {
		return nil, fmt.Errorf("llm client: JSON schema name is required")
	}
	if len(schema.Schema) == 0 || !json.Valid(schema.Schema) {
		return nil, fmt.Errorf("llm client: JSON schema must be valid JSON")
	}
	var document any
	if err := json.Unmarshal(schema.Schema, &document); err != nil {
		return nil, fmt.Errorf("llm client: decode JSON schema: %w", err)
	}
	if _, ok := document.(map[string]any); !ok {
		return nil, fmt.Errorf("llm client: JSON schema must be an object")
	}
	return &ResponseFormat{Type: "json_schema", JSONSchema: &ResponseFormatJSONSchema{Name: schema.Name, Strict: true, Schema: schema.Schema}}, nil
}

// ResponseFormat is the OpenAI-compatible response-format request object.
type ResponseFormat struct {
	Type       string                    `json:"type"`
	JSONSchema *ResponseFormatJSONSchema `json:"json_schema,omitempty"`
}

// ResponseFormatJSONSchema is the strict JSON Schema payload accepted by
// OpenAI-compatible chat-completions providers.
type ResponseFormatJSONSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
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
	profile               config.ModelProfile
	transport             http.RoundTripper
	suppressCompletionLog bool
	allowEmptyFinal       bool
}

// NewEvaluationClient creates a client for private evaluation work.  Provider
// response metadata is untrusted, so evaluation must not put it in ordinary
// application logs.
func NewEvaluationClient(profile config.ModelProfile) *Client {
	client := NewClient(profile)
	client.suppressCompletionLog = true
	client.allowEmptyFinal = true
	return client
}

// NewClient constructs the one LLM client from a validated fixed scope.
func NewClient(profile config.ModelProfile) *Client {
	return &Client{
		profile:   profile,
		transport: http.DefaultTransport,
	}
}

// Chat sends one OpenAI-compatible Chat Completions request and validates the
// provider result before application code receives it.
func (c *Client) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	return c.chat(ctx, messages, nil)
}

// ChatWithJSONSchema sends one request with a strict JSON Schema response
// contract. It shares the ordinary Chat transport and response validation path.
func (c *Client) ChatWithJSONSchema(ctx context.Context, messages []ChatMessage, schema JSONSchema) (*ChatResponse, error) {
	format, err := schema.responseFormat()
	if err != nil {
		return nil, err
	}
	return c.chat(ctx, messages, format)
}

func (c *Client) chat(ctx context.Context, messages []ChatMessage, responseFormat *ResponseFormat) (*ChatResponse, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("llm client: messages are required")
	}
	requestBody, err := json.Marshal(ChatRequest{
		Model:           c.profile.Model,
		Messages:        messages,
		Temperature:     c.profile.Temperature,
		MaxTokens:       c.profile.MaxTokens,
		ReasoningEffort: c.profile.ReasoningEffort,
		TopP:            c.profile.TopP,
		TopK:            c.profile.TopK,
		MinP:            c.profile.MinP,
		PresencePenalty: c.profile.PresencePenalty,
		RepeatPenalty:   c.profile.RepeatPenalty,
		ResponseFormat:  responseFormat,
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
		if responseFormat != nil && errors.Is(err, ErrRequestRejected) {
			return nil, fmt.Errorf("llm client: %w: %v", ErrStructuredRequestRejected, err)
		}
		return nil, err
	}

	var response ChatResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("llm client: decode chat response: %w", err)
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("llm client: %w: chat response has no content", ErrUnusableResponse)
	}
	if strings.TrimSpace(response.Choices[0].Message.Content) == "" && !c.allowEmptyFinal {
		return nil, fmt.Errorf("llm client: %w: chat response has no final content", ErrUnusableResponse)
	}
	if !c.suppressCompletionLog {
		logging.Info("Chat completed", "model", response.Model, "tokens", response.Usage.TotalTokens)
	}
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
	// RoundTrip performs exactly one exchange. http.Client.Do parses Location
	// before CheckRedirect, which can expose an invalid redirect target in its
	// error. A one-hop transport path makes every 3xx response safe to classify
	// by status without inspecting or following its Location header.
	timed, cancel := context.WithTimeout(request.Context(), providerRequestTimeout)
	defer cancel()
	transport := c.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	response, err := transport.RoundTrip(request.Clone(timed))
	if err != nil {
		return nil, fmt.Errorf("llm client: request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < 400 {
		return nil, providerRedirectError(response.StatusCode)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, providerStatusError(response.StatusCode)
	}
	data, err := readProviderBody(response.Body)
	if err != nil {
		return nil, err
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

func providerStatusError(status int) error {
	if status == http.StatusBadRequest {
		return fmt.Errorf("llm client: %w (status %d)", ErrRequestRejected, status)
	}
	return fmt.Errorf("llm client: provider returned status %d", status)
}

func providerRedirectError(status int) error {
	return fmt.Errorf("llm client: %w (status %d)", ErrRedirectRejected, status)
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
