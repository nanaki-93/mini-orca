package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter_FullFlow_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			resp := listModelsResponse{
				Object: "list",
				Data: []modelEntry{
					{ID: "model-a", Object: "model", OwnedBy: "owner-a"},
					{ID: "model-b", Object: "model", OwnedBy: "owner-b"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	router := NewRouter()
	router.RegisterProvider(provider)

	models, err := router.RouteListModels(context.Background())
	if err != nil {
		t.Fatalf("RouteListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].ID != "model-a" {
		t.Errorf("expected model-a, got %s", models[0].ID)
	}
}

func TestRouter_FullFlow_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}
			if req.Model != "test-model" {
				t.Errorf("expected model test-model, got %s", req.Model)
			}

			resp := ChatResponse{
				ID:      "chat-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "test-model",
				Choices: []ChatChoice{
					{Index: 0, Message: ChatMessage{Role: "assistant", Content: "Hello from LM Studio!"}, FinishReason: "stop"},
				},
				Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	router := NewRouter()
	router.RegisterProvider(provider)
	router.SetDefaultConfig("planning", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "test-model",
		Temperature: 0.7,
		MaxTokens:   1024,
	})

	result, err := router.Chat("planning", []ChatMessage{{Role: "user", Content: "Hello"}})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if result.ID != "chat-1" {
		t.Errorf("expected chat-1, got %s", result.ID)
	}
	if result.Model != "test-model" {
		t.Errorf("expected model test-model, got %s", result.Model)
	}
	if len(result.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(result.Choices))
	}
	if result.Choices[0].Message.Content != "Hello from LM Studio!" {
		t.Errorf("expected 'Hello from LM Studio!', got %s", result.Choices[0].Message.Content)
	}
}

func TestRouter_FullFlow_RouteChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := ChatResponse{
				ID:      "chat-2",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "route-model",
				Choices: []ChatChoice{
					{Index: 0, Message: ChatMessage{Role: "assistant", Content: "Routed!"}, FinishReason: "stop"},
				},
				Usage: ChatUsage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	router := NewRouter()
	router.RegisterProvider(provider)
	router.SetDefaultConfig("coding", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "route-model",
		Temperature: 0.5,
		MaxTokens:   2048,
	})

	result, err := router.RouteChat(context.Background(), "coding", []ChatMessage{{Role: "user", Content: "Write code"}})
	if err != nil {
		t.Fatalf("RouteChat failed: %v", err)
	}

	if result.ID != "chat-2" {
		t.Errorf("expected chat-2, got %s", result.ID)
	}
}

func TestRouter_FullFlow_FallbackToActive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := ChatResponse{
				ID:      "chat-3",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "active-model",
				Choices: []ChatChoice{
					{Index: 0, Message: ChatMessage{Role: "assistant", Content: "Fallback!"}, FinishReason: "stop"},
				},
				Usage: ChatUsage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	router := NewRouter()
	router.RegisterProvider(provider)
	// No phase config set — should fall back to active provider
	router.SetActiveProvider(provider.Name())

	result, err := router.Chat("unknown_phase", []ChatMessage{{Role: "user", Content: "Test"}})
	if err != nil {
		t.Fatalf("Chat fallback failed: %v", err)
	}

	if result.Model != "active-model" {
		t.Errorf("expected active-model, got %s", result.Model)
	}
}

func TestRouter_FullFlow_GetPhaseConfig(t *testing.T) {
	router := NewRouter()
	router.SetDefaultConfig("planning", ModelConfig{
		Provider:    "lm-studio",
		ModelID:     "planning-model",
		Temperature: 0.3,
		MaxTokens:   1024,
	})

	phaseCfg, err := router.GetPhaseConfig("planning")
	if err != nil {
		t.Fatalf("GetPhaseConfig failed: %v", err)
	}

	if phaseCfg.ModelID != "planning-model" {
		t.Errorf("expected planning-model, got %s", phaseCfg.ModelID)
	}
	if phaseCfg.Temperature != 0.3 {
		t.Errorf("expected 0.3, got %f", phaseCfg.Temperature)
	}
}

func TestRouter_FullFlow_UnconfiguredPhase(t *testing.T) {
	router := NewRouter()
	_, err := router.GetPhaseConfig("nonexistent")
	if err == nil {
		t.Fatal("expected error for unconfigured phase")
	}
}
