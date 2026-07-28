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

func TestNewPlannerAgent(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()

	agent := NewPlannerAgent(router, registry)
	if agent == nil {
		t.Fatal("expected non-nil planner agent")
	}
	if agent.Name() != "planner" {
		t.Errorf("expected name 'planner', got %s", agent.Name())
	}
	if agent.Description() != "Plans and breaks down tasks into actionable subtasks" {
		t.Errorf("unexpected description: %s", agent.Description())
	}
	if agent.Phase() != model.PhasePlanning {
		t.Errorf("expected phase %s, got %s", model.PhasePlanning, agent.Phase())
	}
}

func TestPlannerAgent_Execute_NoRouter(t *testing.T) {
	registry := skills.NewSkillsRegistry()
	agent := NewPlannerAgent(nil, registry)

	_, err := agent.Execute(context.Background(), "Plan this task")
	if err == nil {
		t.Fatal("expected error for nil router")
	}
}

func TestPlannerAgent_Execute_EmptyInput(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	agent := NewPlannerAgent(router, registry)

	_, err := agent.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestPlannerAgent_Execute_NoRegistry(t *testing.T) {
	router := model.NewRouter()
	agent := NewPlannerAgent(router, nil)

	_, err := agent.Execute(context.Background(), "Plan this task")
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

func TestPlannerAgent_Execute_FullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			resp := model.ChatResponse{
				ID:      "planner-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "planning-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "1. Analyze requirements\n2. Design architecture\n3. Implement core logic\n4. Write tests\n5. Deploy"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 10, CompletionTokens: 50, TotalTokens: 60},
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
		Temperature: 0.7,
		MaxTokens:   4096,
	})

	registry := skills.NewSkillsRegistry()
	agent := NewPlannerAgent(router, registry)

	result, err := agent.Execute(context.Background(), "Build a REST API for user management")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if result.Phase != "planning" {
		t.Errorf("expected phase 'planning', got %s", result.Phase)
	}
	if result.Metadata["model"] != "planning-model" {
		t.Errorf("expected model 'planning-model', got %s", result.Metadata["model"])
	}
	if result.Metadata["tokens"] != "60" {
		t.Errorf("expected tokens '60', got %s", result.Metadata["tokens"])
	}
}

func TestPlannerAgent_Execute_WithSkills(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			resp := model.ChatResponse{
				ID:      "planner-2",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "planning-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "Plan generated with skills"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 20, CompletionTokens: 30, TotalTokens: 50},
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
		Temperature: 0.7,
		MaxTokens:   4096,
	})

	registry := skills.NewSkillsRegistry()
	agent := NewPlannerAgent(router, registry)

	// Set planner-specific skills
	agent.SetSkills([]string{"task_breakdown", "solid_principles", "architecture_design"})

	result, err := agent.Execute(context.Background(), "Build a microservice")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if result.Phase != "planning" {
		t.Errorf("expected phase 'planning', got %s", result.Phase)
	}
}

func TestPlannerAgent_GetSkills(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	agent := NewPlannerAgent(router, registry)

	// Default skills should be empty
	skills := agent.GetSkills()
	if skills == nil {
		t.Fatal("expected non-nil skills slice")
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}

	// Set skills
	agent.SetSkills([]string{"task_breakdown", "solid_principles"})
	skills = agent.GetSkills()
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}
