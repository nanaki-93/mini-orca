package model

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

// LMStudioProvider implements the Provider interface for LM Studio.
type LMStudioProvider struct {
	baseURL    string
	httpClient *http.Client
}

// NewLMStudioProvider creates a new LMStudioProvider instance.
func NewLMStudioProvider(baseURL string) *LMStudioProvider {
	return &LMStudioProvider{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

// listModelsResponse represents the JSON response from LM Studio's /v1/models endpoint.
type listModelsResponse struct {
	Object string       `json:"object"`
	Data   []modelEntry `json:"data"`
}

// modelEntry represents a single model entry in the /v1/models response.
type modelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// ListModels calls LM Studio's /v1/models endpoint and returns the list of available models.
func (p *LMStudioProvider) ListModels(ctx context.Context) ([]Model, error) {
	url := fmt.Sprintf("%s/v1/models", p.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logging.Error("Failed to create request", "provider", "lm-studio", "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		logging.Error("Request failed", "provider", "lm-studio", "error", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logging.Error("Unexpected status code", "provider", "lm-studio", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var listResp listModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		logging.Error("Failed to decode response", "provider", "lm-studio", "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]Model, 0, len(listResp.Data))
	for _, entry := range listResp.Data {
		models = append(models, Model{
			ID:      entry.ID,
			Object:  entry.Object,
			OwnedBy: entry.OwnedBy,
		})
	}

	logging.Info("Listed models", "provider", "lm-studio", "count", len(models))
	return models, nil
}

// Chat sends a chat completion request to LM Studio.
func (p *LMStudioProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if req.Stream {
		return nil, fmt.Errorf("streaming not yet supported")
	}

	url := fmt.Sprintf("%s/v1/chat/completions", p.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		logging.Error("Failed to marshal request", "provider", "lm-studio", "error", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		logging.Error("Failed to create request", "provider", "lm-studio", "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(request)
	if err != nil {
		logging.Error("Request failed", "provider", "lm-studio", "error", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logging.Error("Unexpected status code", "provider", "lm-studio", "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		logging.Error("Failed to decode response", "provider", "lm-studio", "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	logging.Info("Chat completed", "provider", "lm-studio", "model", chatResp.Model, "tokens", chatResp.Usage.TotalTokens)
	return &chatResp, nil
}

// Name returns the name of the provider.
func (p *LMStudioProvider) Name() string {
	return "lm-studio"
}

// IsStreamingSupported returns whether the provider supports streaming responses.
func (p *LMStudioProvider) IsStreamingSupported() bool {
	return false
}
