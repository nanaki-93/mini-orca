package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func TestNewCoderAgent(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)

	agent := NewCoderAgent(client)
	if agent == nil {
		t.Fatal("expected non-nil coder agent")
	}
	if agent.Name() != "coder" {
		t.Errorf("expected name 'coder', got %s", agent.Name())
	}
	if agent.Description() != "Implements code based on plans and specifications" {
		t.Errorf("unexpected description: %s", agent.Description())
	}
	if agent.Phase() != "coding" {
		t.Errorf("expected phase 'coding', got %s", agent.Phase())
	}
}

func TestCoderAgent_Execute_NoClient(t *testing.T) {
	agent := NewCoderAgent(nil)

	_, err := agent.Execute(context.Background(), "Implement a User struct")
	if err == nil {
		t.Fatal("expected error for nil LLM client")
	}
}

func TestCoderAgent_Execute_EmptyInput(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	agent := NewCoderAgent(client)

	_, err := agent.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestCoderAgent_Execute_FullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req llm.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			resp := llm.ChatResponse{
				ID:      "coder-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "coding-model",
				Choices: []llm.ChatChoice{
					{
						Index:        0,
						Message:      llm.ChatMessage{Role: "assistant", Content: "package user\n\ntype User struct {\n\tID    string `json:\"id\"`\n\tName  string `json:\"name\"`\n\tEmail string `json:\"email\"`\n}"},
						FinishReason: "stop",
					},
				},
				Usage: llm.ChatUsage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test-key", "coding-model", 0.3, 4096)
	agent := NewCoderAgent(client)

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
			var req llm.ChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			resp := llm.ChatResponse{
				ID:      "coder-2",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "coding-model",
				Choices: []llm.ChatChoice{
					{
						Index:        0,
						Message:      llm.ChatMessage{Role: "assistant", Content: "func NewUser(id, name, email string) *User {\n\treturn &User{ID: id, Name: name, Email: email}\n}"},
						FinishReason: "stop",
					},
				},
				Usage: llm.ChatUsage{PromptTokens: 60, CompletionTokens: 80, TotalTokens: 140},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test-key", "coding-model", 0.3, 4096)
	agent := NewCoderAgent(client)

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
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	agent := NewCoderAgent(client)

	// Default skills should be empty
	agentSkills := agent.GetSkills()
	if agentSkills == nil {
		t.Fatal("expected non-nil skills slice")
	}
	if len(agentSkills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(agentSkills))
	}

	// Set skills
	agent.SetSkills([]string{"function_generation", "clean_code"})
	agentSkills = agent.GetSkills()
	if len(agentSkills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(agentSkills))
	}
}
