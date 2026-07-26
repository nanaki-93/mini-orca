package model

import "net/http"

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
