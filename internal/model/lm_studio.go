package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
		httpClient: http.DefaultClient,
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
		log.Printf("[lm-studio] failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		log.Printf("[lm-studio] request failed: %v", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[lm-studio] unexpected status code: %d, body: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var listResp listModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		log.Printf("[lm-studio] failed to decode response: %v", err)
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

	log.Printf("[lm-studio] listed %d models", len(models))
	return models, nil
}
