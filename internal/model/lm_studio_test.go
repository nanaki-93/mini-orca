package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLMStudioProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/models" {
			t.Errorf("expected /v1/models, got %s", r.URL.Path)
		}

		resp := listModelsResponse{
			Object: "list",
			Data: []modelEntry{
				{ID: "model-1", Object: "model", OwnedBy: "owner-1"},
				{ID: "model-2", Object: "model", OwnedBy: "owner-2"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	models, err := provider.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	if models[0].ID != "model-1" {
		t.Errorf("expected model-1, got %s", models[0].ID)
	}
	if models[1].ID != "model-2" {
		t.Errorf("expected model-2, got %s", models[1].ID)
	}
}

func TestLMStudioProvider_ListModels_HttpError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	_, err := provider.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLMStudioProvider_ListModels_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	_, err := provider.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLMStudioProvider_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("expected test-model, got %s", req.Model)
		}

		resp := ChatResponse{
			ID:      "resp-1",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "test-model",
			Choices: []ChatChoice{
				{Index: 0, Message: ChatMessage{Role: "assistant", Content: "Hello!"}, FinishReason: "stop"},
			},
			Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	result, err := provider.Chat(context.Background(), ChatRequest{
		Model:       "test-model",
		Messages:    []ChatMessage{{Role: "user", Content: "Hi"}},
		Temperature: 0.7,
		MaxTokens:   100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != "resp-1" {
		t.Errorf("expected resp-1, got %s", result.ID)
	}
	if result.Model != "test-model" {
		t.Errorf("expected test-model, got %s", result.Model)
	}
	if len(result.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(result.Choices))
	}
	if result.Choices[0].Message.Content != "Hello!" {
		t.Errorf("expected Hello!, got %s", result.Choices[0].Message.Content)
	}
	if result.Usage.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", result.Usage.TotalTokens)
	}
}

func TestLMStudioProvider_Chat_StreamingNotSupported(t *testing.T) {
	provider := NewLMStudioProvider("http://localhost:1234")
	_, err := provider.Chat(context.Background(), ChatRequest{
		Model:  "test-model",
		Stream: true,
	})
	if err == nil {
		t.Fatal("expected error for streaming, got nil")
	}
	if err.Error() != "streaming not yet supported" {
		t.Errorf("expected 'streaming not yet supported', got %q", err.Error())
	}
}

func TestLMStudioProvider_Chat_HttpError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	_, err := provider.Chat(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLMStudioProvider_Chat_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	_, err := provider.Chat(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLMStudioProvider_Name(t *testing.T) {
	provider := NewLMStudioProvider("http://localhost:1234")
	if got := provider.Name(); got != "lm-studio" {
		t.Errorf("expected lm-studio, got %s", got)
	}
}

func TestLMStudioProvider_IsStreamingSupported(t *testing.T) {
	provider := NewLMStudioProvider("http://localhost:1234")
	if got := provider.IsStreamingSupported(); got != false {
		t.Errorf("expected false, got %v", got)
	}
}

func TestLMStudioProvider_ListModels_Integration(t *testing.T) {
	t.Skip("requires local LM Studio running on http://localhost:1234")

	provider := NewLMStudioProvider("http://localhost:1234")
	models, err := provider.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) == 0 {
		t.Error("expected at least one model")
	}
}

func TestLMStudioProvider_Chat_Integration(t *testing.T) {
	t.Skip("requires local LM Studio running on http://localhost:1234")

	provider := NewLMStudioProvider("http://localhost:1234")
	result, err := provider.Chat(context.Background(), ChatRequest{
		Model:    "local-model",
		Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Choices) == 0 {
		t.Error("expected non-empty response")
	}
}
