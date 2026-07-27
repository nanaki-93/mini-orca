package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nanaki-93/mini-orca/internal/model"
)

func TestClient_FullFlow_Generate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			resp := model.ChatResponse{
				ID:      "full-flow-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "agent-model",
				Choices: []model.ChatChoice{
					{Index: 0, Message: model.ChatMessage{Role: "assistant", Content: "Agent response!"}, FinishReason: "stop"},
				},
				Usage: model.ChatUsage{PromptTokens: 8, CompletionTokens: 12, TotalTokens: 20},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := model.NewLMStudioProvider(server.URL)
	router := model.NewRouter()
	router.RegisterProvider(provider)
	router.SetDefaultConfig("planning", model.ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "agent-model",
		Temperature: 0.7,
		MaxTokens:   4096,
	})

	client := NewClient(router)
	result, err := client.Generate("planning", []model.ChatMessage{{Role: "user", Content: "Plan this"}})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.ID != "full-flow-1" {
		t.Errorf("expected full-flow-1, got %s", result.ID)
	}
	if result.Model != "agent-model" {
		t.Errorf("expected agent-model, got %s", result.Model)
	}
	if len(result.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(result.Choices))
	}
	if result.Choices[0].Message.Content != "Agent response!" {
		t.Errorf("expected 'Agent response!', got %s", result.Choices[0].Message.Content)
	}
}

func TestClient_FullFlow_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			resp := map[string]any{
				"object": "list",
				"data": []map[string]string{
					{"id": "model-1", "object": "model", "owned_by": "owner-1"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := model.NewLMStudioProvider(server.URL)
	router := model.NewRouter()
	router.RegisterProvider(provider)

	client := NewClient(router)
	models, err := client.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].ID != "model-1" {
		t.Errorf("expected model-1, got %s", models[0].ID)
	}
}

func TestClient_FullFlow_MultiplePhases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			resp := model.ChatResponse{
				ID:      "multi-phase",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   req.Model,
				Choices: []model.ChatChoice{
					{Index: 0, Message: model.ChatMessage{Role: "assistant", Content: "OK"}, FinishReason: "stop"},
				},
				Usage: model.ChatUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := model.NewLMStudioProvider(server.URL)
	router := model.NewRouter()
	router.RegisterProvider(provider)
	router.SetDefaultConfig("planning", model.ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "planning-model",
		Temperature: 0.3,
		MaxTokens:   1024,
	})
	router.SetDefaultConfig("coding", model.ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "coding-model",
		Temperature: 0.7,
		MaxTokens:   4096,
	})

	client := NewClient(router)

	// Test planning phase
	result, err := client.Generate("planning", []model.ChatMessage{{Role: "user", Content: "Plan"}})
	if err != nil {
		t.Fatalf("planning Generate failed: %v", err)
	}
	if result.Model != "planning-model" {
		t.Errorf("expected planning-model, got %s", result.Model)
	}

	// Test coding phase
	result, err = client.Generate("coding", []model.ChatMessage{{Role: "user", Content: "Code"}})
	if err != nil {
		t.Fatalf("coding Generate failed: %v", err)
	}
	if result.Model != "coding-model" {
		t.Errorf("expected coding-model, got %s", result.Model)
	}
}
