package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nanaki-93/mini-orca/internal/agent/skills"
	"github.com/nanaki-93/mini-orca/internal/model"
)

func TestNewCoderAgent(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()

	agent := NewCoderAgent(router, registry)
	if agent == nil {
		t.Fatal("expected non-nil coder agent")
	}
	if agent.Name() != "coder" {
		t.Errorf("expected name 'coder', got %s", agent.Name())
	}
	if agent.Description() != "Implements code based on plans and specifications" {
		t.Errorf("unexpected description: %s", agent.Description())
	}
	if agent.Phase() != model.PhaseCoding {
		t.Errorf("expected phase %s, got %s", model.PhaseCoding, agent.Phase())
	}
}

func TestCoderAgent_Execute_NoRouter(t *testing.T) {
	registry := skills.NewSkillsRegistry()
	agent := NewCoderAgent(nil, registry)

	_, err := agent.Execute(context.Background(), "Implement a User struct")
	if err == nil {
		t.Fatal("expected error for nil router")
	}
}

func TestCoderAgent_Execute_EmptyInput(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	agent := NewCoderAgent(router, registry)

	_, err := agent.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestCoderAgent_Execute_NoRegistry(t *testing.T) {
	router := model.NewRouter()
	agent := NewCoderAgent(router, nil)

	_, err := agent.Execute(context.Background(), "Implement a User struct")
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

func TestCoderAgent_Execute_FullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			resp := model.ChatResponse{
				ID:      "coder-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "coding-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "package user\n\ntype User struct {\n\tID    string `json:\"id\"`\n\tName  string `json:\"name\"`\n\tEmail string `json:\"email\"`\n}"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150},
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
	router.SetDefaultConfig("coding", model.ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "coding-model",
		Temperature: 0.3,
		MaxTokens:   4096,
	})

	registry := skills.NewSkillsRegistry()
	agent := NewCoderAgent(router, registry)

	result, err := agent.Execute(context.Background(), "Implement a User struct with ID, Name, Email fields and JSON tags")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if result.Phase != "coding" {
		t.Errorf("expected phase 'coding', got %s", result.Phase)
	}
	if result.Metadata["model"] != "coding-model" {
		t.Errorf("expected model 'coding-model', got %s", result.Metadata["model"])
	}
	if result.Metadata["tokens"] != "150" {
		t.Errorf("expected tokens '150', got %s", result.Metadata["tokens"])
	}
}

func TestCoderAgent_Execute_WithSkills(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			resp := model.ChatResponse{
				ID:      "coder-2",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "coding-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "func NewUser(id, name, email string) *User {\n\treturn &User{ID: id, Name: name, Email: email}\n}"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 60, CompletionTokens: 80, TotalTokens: 140},
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
	router.SetDefaultConfig("coding", model.ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "coding-model",
		Temperature: 0.3,
		MaxTokens:   4096,
	})

	registry := skills.NewSkillsRegistry()
	agent := NewCoderAgent(router, registry)

	// Set coder-specific skills
	agent.SetSkills([]string{"function_generation", "struct_design", "clean_code"})

	result, err := agent.Execute(context.Background(), "Implement a constructor function NewUser for the User struct")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if result.Phase != "coding" {
		t.Errorf("expected phase 'coding', got %s", result.Phase)
	}
}

func TestCoderAgent_GetSkills(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	agent := NewCoderAgent(router, registry)

	// Default skills should be empty
	skills := agent.GetSkills()
	if skills == nil {
		t.Fatal("expected non-nil skills slice")
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}

	// Set skills
	agent.SetSkills([]string{"function_generation", "clean_code"})
	skills = agent.GetSkills()
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}
