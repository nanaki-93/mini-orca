package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/agent/skills"
	"github.com/nanaki-93/mini-orca/v2/internal/model"
)

// TestCoderAgent_PromptBuilding verifies that the coder agent builds prompts
// correctly with atomic unit descriptions.
func TestCoderAgent_PromptBuilding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			content := req.Messages[0].Content

			// Verify the prompt contains the atomic unit description marker
			if !strings.Contains(content, "Atomic Unit Description:") {
				t.Error("expected prompt to contain 'Atomic Unit Description:' marker")
			}
			if !strings.Contains(content, "User struct") {
				t.Error("expected prompt to contain atomic unit description")
			}

			resp := model.ChatResponse{
				ID:      "coder-prompt-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "coding-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "package user\n\ntype User struct {\n\tID string\n}"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 50, CompletionTokens: 30, TotalTokens: 80},
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

	_, err := agent.Execute(context.Background(), "Create a User struct with ID, Name, Email fields")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

// TestCoderAgent_SkillsInPrompt verifies that skills are correctly included
// in the coder's prompt when skills are set.
func TestCoderAgent_SkillsInPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			content := req.Messages[0].Content

			// Verify that the prompt contains the atomic unit description
			if !strings.Contains(content, "constructor function") {
				t.Error("expected prompt to contain atomic unit description")
			}

			resp := model.ChatResponse{
				ID:      "coder-prompt-2",
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
				Usage: model.ChatUsage{PromptTokens: 60, CompletionTokens: 40, TotalTokens: 100},
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
	agent.SetSkills([]string{"code_generation", "refactoring", "clean_code"})

	_, err := agent.Execute(context.Background(), "Implement a constructor function NewUser for the User struct")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify skills are set
	if len(agent.GetSkills()) != 3 {
		t.Errorf("expected 3 skills, got %d", len(agent.GetSkills()))
	}
}

// TestTesterAgent_PromptBuilding verifies that the tester agent builds prompts
// correctly with code and test results.
func TestTesterAgent_PromptBuilding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			content := req.Messages[0].Content

			// Verify the prompt contains the code and test results marker
			if !strings.Contains(content, "Code and Test Results:") {
				t.Error("expected prompt to contain 'Code and Test Results:' marker")
			}
			if !strings.Contains(content, "TestUser") {
				t.Error("expected prompt to contain test code")
			}

			resp := model.ChatResponse{
				ID:      "tester-prompt-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "testing-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "## Summary\nAll tests passed.\n\n## Coverage: 90%"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 80, CompletionTokens: 30, TotalTokens: 110},
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
	router.SetDefaultConfig("testing", model.ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "testing-model",
		Temperature: 0.2,
		MaxTokens:   4096,
	})

	registry := skills.NewSkillsRegistry()
	agent := NewTesterAgent(router, registry)

	_, err := agent.Execute(context.Background(), "package user\n\nfunc TestUser(t *testing.T) {\n\t// test code\n}")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

// TestTesterAgent_SkillsInPrompt verifies that skills are correctly included
// in the tester's prompt when skills are set.
func TestTesterAgent_SkillsInPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			content := req.Messages[0].Content

			// Verify that the prompt contains code and test results marker
			if !strings.Contains(content, "Code and Test Results:") {
				t.Error("expected prompt to contain 'Code and Test Results:' marker")
			}

			resp := model.ChatResponse{
				ID:      "tester-prompt-2",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "testing-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "## Summary\nAll tests passed.\n\n## Coverage: 85%"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 70, CompletionTokens: 25, TotalTokens: 95},
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
	router.SetDefaultConfig("testing", model.ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "testing-model",
		Temperature: 0.2,
		MaxTokens:   4096,
	})

	registry := skills.NewSkillsRegistry()
	agent := NewTesterAgent(router, registry)

	// Set tester-specific skills
	agent.SetSkills([]string{"test_generation", "edge_case_detection", "validation"})

	_, err := agent.Execute(context.Background(), "test input with skills")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify skills are set
	if len(agent.GetSkills()) != 3 {
		t.Errorf("expected 3 skills, got %d", len(agent.GetSkills()))
	}
}

// TestReviewerAgent_PromptBuilding verifies that the reviewer agent builds prompts
// correctly with code and plan content.
func TestReviewerAgent_PromptBuilding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			content := req.Messages[0].Content

			// Verify the prompt contains the code to review marker
			if !strings.Contains(content, "Code to Review:") {
				t.Error("expected prompt to contain 'Code to Review:' marker")
			}
			if !strings.Contains(content, "NewUser") {
				t.Error("expected prompt to contain code content")
			}

			resp := model.ChatResponse{
				ID:      "reviewer-prompt-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "review-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "## Summary\nCode quality is good.\n\n## Score: 85\n\n## Recommendation\nApprove"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 80, CompletionTokens: 30, TotalTokens: 110},
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
	router.SetDefaultConfig("review", model.ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "review-model",
		Temperature: 0.2,
		MaxTokens:   4096,
	})

	registry := skills.NewSkillsRegistry()
	agent := NewReviewerAgent(router, registry)

	_, err := agent.Execute(context.Background(), "package user\n\nfunc NewUser(id, name string) *User {\n\treturn &User{ID: id, Name: name}\n}")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

// TestReviewerAgent_SkillsInPrompt verifies that skills are correctly included
// in the reviewer's prompt when skills are set.
func TestReviewerAgent_SkillsInPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			content := req.Messages[0].Content

			// Verify that the prompt contains code to review marker
			if !strings.Contains(content, "Code to Review:") {
				t.Error("expected prompt to contain 'Code to Review:' marker")
			}

			resp := model.ChatResponse{
				ID:      "reviewer-prompt-2",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "review-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "## Summary\nCode follows best practices.\n\n## Score: 92\n\n## Recommendation\nApprove"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 70, CompletionTokens: 25, TotalTokens: 95},
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
	router.SetDefaultConfig("review", model.ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "review-model",
		Temperature: 0.2,
		MaxTokens:   4096,
	})

	registry := skills.NewSkillsRegistry()
	agent := NewReviewerAgent(router, registry)

	// Set reviewer-specific skills
	agent.SetSkills([]string{"code_review", "security_check", "best_practices"})

	_, err := agent.Execute(context.Background(), "test input with skills")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify skills are set
	if len(agent.GetSkills()) != 3 {
		t.Errorf("expected 3 skills, got %d", len(agent.GetSkills()))
	}
}

// TestAgent_EmptySkillsFallback verifies that agents work correctly with empty skill sets.
func TestAgent_EmptySkillsFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			content := req.Messages[0].Content

			// When no skills are registered, the agent should use a fallback prompt
			if !strings.Contains(content, "Goal:") && !strings.Contains(content, "Atomic Unit Description:") &&
				!strings.Contains(content, "Code and Test Results:") && !strings.Contains(content, "Code to Review:") {
				t.Error("expected prompt to contain content markers")
			}

			resp := model.ChatResponse{
				ID:      "fallback-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "coding-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "Fallback response"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20},
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
		Temperature: 0.7,
		MaxTokens:   4096,
	})

	// Create an empty registry (no skills registered)
	registry := skills.NewSkillsRegistry()
	agent := NewCoderAgent(router, registry)

	// Don't set any skills - should use fallback
	_, err := agent.Execute(context.Background(), "Build a REST API")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify skills are empty
	if len(agent.GetSkills()) != 0 {
		t.Errorf("expected 0 skills, got %d", len(agent.GetSkills()))
	}
}

// TestAgent_SkillsRegistryIntegration verifies that skills from the registry
// are correctly used by agents.
func TestAgent_SkillsRegistryIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			content := req.Messages[0].Content

			// Verify the prompt contains goal content
			if !strings.Contains(content, "microservice") {
				t.Error("expected prompt to contain goal content")
			}

			resp := model.ChatResponse{
				ID:      "integration-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "coding-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "Plan with registry skills"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 15, CompletionTokens: 10, TotalTokens: 25},
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
		Temperature: 0.7,
		MaxTokens:   4096,
	})

	// Create a registry with some skills
	registry := skills.NewSkillsRegistry()

	// Register planner-specific skills
	_ = registry.Register(skills.Skill{
		Name:           "code_generation",
		Type:           skills.Knowledge,
		PromptTemplate: "Generate clean, well-documented code",
	})
	_ = registry.Register(skills.Skill{
		Name:           "refactoring",
		Type:           skills.Knowledge,
		PromptTemplate: "Refactor code for clarity and maintainability",
	})

	agent := NewCoderAgent(router, registry)

	// Set skills from the registry
	agent.SetSkills([]string{"task_breakdown", "architecture_design"})

	_, err := agent.Execute(context.Background(), "Build a microservice for user authentication")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify skills are set correctly
	skills := agent.GetSkills()
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}
