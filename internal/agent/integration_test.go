package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/model"
)

func TestClient_FullFlow_Execute(t *testing.T) {
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
	client.name = "test-agent"
	client.phase = model.PhaseCoding

	result, err := client.Execute(context.Background(), "Plan this")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Output != "Agent response!" {
		t.Errorf("expected 'Agent response!', got %s", result.Output)
	}
	if result.Phase != "coding" {
		t.Errorf("expected phase 'coding', got %s", result.Phase)
	}
	if result.Metadata["model"] != "agent-model" {
		t.Errorf("expected model 'agent-model', got %s", result.Metadata["model"])
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
	client.name = "test-agent"

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
	client.name = "test-agent"

	// Test coding phase
	client.phase = model.PhaseCoding
	result, err := client.Execute(context.Background(), "Code")
	if err != nil {
		t.Fatalf("coding Execute failed: %v", err)
	}
	if result.Metadata["model"] != "coding-model" {
		t.Errorf("expected coding-model, got %s", result.Metadata["model"])
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := NewRegistry()
	client := NewClient(nil)
	client.name = "test-agent"

	if err := registry.Register("test-agent", client); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := registry.Get("test-agent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name() != "test-agent" {
		t.Errorf("expected test-agent, got %s", got.Name())
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewRegistry()
	client := NewClient(nil)
	client.name = "test-agent"

	_ = registry.Register("test-agent", client)
	err := registry.Register("test-agent", client)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	client1 := NewClient(nil)
	client1.name = "agent-1"
	_ = registry.Register("agent-1", client1)

	client2 := NewClient(nil)
	client2.name = "agent-2"
	_ = registry.Register("agent-2", client2)

	names := registry.List()
	if len(names) != 2 {
		t.Errorf("expected 2 agent names, got %d", len(names))
	}
	if names[0] != "agent-1" && names[1] != "agent-1" {
		t.Errorf("expected agent-1 in names, got %v", names)
	}
	if names[0] != "agent-2" && names[1] != "agent-2" {
		t.Errorf("expected agent-2 in names, got %v", names)
	}
}

func TestAgentResult_Metadata(t *testing.T) {
	result := &AgentResult{
		Output:   "test output",
		Phase:    "planning",
		Metadata: map[string]string{"key": "value"},
	}

	if result.Output != "test output" {
		t.Errorf("expected 'test output', got %s", result.Output)
	}
	if result.Phase != "planning" {
		t.Errorf("expected phase 'planning', got %s", result.Phase)
	}
	if result.Metadata["key"] != "value" {
		t.Errorf("expected value for key, got %s", result.Metadata["key"])
	}
}

func TestRegistry_NameMismatch(t *testing.T) {
	registry := NewRegistry()
	client := NewClient(nil)
	client.name = "actual-name"

	err := registry.Register("different-name", client)
	if err == nil {
		t.Fatal("expected error for name mismatch")
	}
}

func TestDefaultAgentNames(t *testing.T) {
	names := DefaultAgentNames()
	expected := []string{"planner", "coder", "tester", "reviewer"}

	if len(names) != len(expected) {
		t.Errorf("expected %d default agent names, got %d", len(expected), len(names))
	}

	for _, name := range expected {
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in default agent names", name)
		}
	}
}
