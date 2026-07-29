package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nanaki-93/mini-orca/internal/agent/prompts"
	"github.com/nanaki-93/mini-orca/internal/agent/skills"
	"github.com/nanaki-93/mini-orca/internal/model"
)

func TestNewOrchestrator(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()

	orch := NewOrchestrator(router, registry, nil)
	if orch == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	if orch.router != router {
		t.Error("expected router to be set")
	}
	if orch.registry != registry {
		t.Error("expected registry to be set")
	}
}

func TestOrchestrator_RunPlanner_EmptyGoal(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(router, registry, nil)

	_, err := orch.RunPlanner("")
	if err == nil {
		t.Fatal("expected error for empty goal, got nil")
	}
}

func TestOrchestrator_RunPlanner_NoRouter(t *testing.T) {
	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(nil, registry, nil)

	_, err := orch.RunPlanner("test goal")
	if err == nil {
		t.Fatal("expected error for nil router, got nil")
	}
}

func TestOrchestrator_RunCoder_EmptyTitle(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(router, registry, nil)

	_, err := orch.RunCoder(prompts.PlanUnit{
		Title:       "",
		Description: "Test description",
	})
	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}
}

func TestOrchestrator_RunCoder_EmptyDescription(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(router, registry, nil)

	_, err := orch.RunCoder(prompts.PlanUnit{
		Title:       "Test",
		Description: "",
	})
	if err == nil {
		t.Fatal("expected error for empty description, got nil")
	}
}

func TestOrchestrator_RunCoder_NoRouter(t *testing.T) {
	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(nil, registry, nil)

	_, err := orch.RunCoder(prompts.PlanUnit{
		Title:       "Test",
		Description: "Test description",
	})
	if err == nil {
		t.Fatal("expected error for nil router, got nil")
	}
}

func TestOrchestrator_RunTester_EmptyCode(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(router, registry, nil)

	_, err := orch.RunTester("", "test results")
	if err == nil {
		t.Fatal("expected error for empty code, got nil")
	}
}

func TestOrchestrator_RunTester_EmptyResults(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(router, registry, nil)

	_, err := orch.RunTester("code", "")
	if err == nil {
		t.Fatal("expected error for empty test results, got nil")
	}
}

func TestOrchestrator_RunTester_NoRouter(t *testing.T) {
	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(nil, registry, nil)

	_, err := orch.RunTester("code", "test results")
	if err == nil {
		t.Fatal("expected error for nil router, got nil")
	}
}

func TestOrchestrator_RunReviewer_EmptyCode(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(router, registry, nil)

	_, err := orch.RunReviewer("", "plan")
	if err == nil {
		t.Fatal("expected error for empty code, got nil")
	}
}

func TestOrchestrator_RunReviewer_EmptyPlan(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(router, registry, nil)

	_, err := orch.RunReviewer("code", "")
	if err == nil {
		t.Fatal("expected error for empty plan, got nil")
	}
}

func TestOrchestrator_RunReviewer_NoRouter(t *testing.T) {
	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(nil, registry, nil)

	_, err := orch.RunReviewer("code", "plan")
	if err == nil {
		t.Fatal("expected error for nil router, got nil")
	}
}

func TestOrchestrator_RunPlanner_FullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := model.ChatResponse{
				ID:      "planner-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "planning-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "1. **Setup Project**\n   Description: Initialize the Go module and project structure\n   Dependencies: None\n\n2. **Implement Core Logic**\n   Description: Implement the main business logic\n   Dependencies: 1"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 50, CompletionTokens: 80, TotalTokens: 130},
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
	orch := NewOrchestrator(router, registry, nil)

	result, err := orch.RunPlanner("Build a REST API for user management")
	if err != nil {
		t.Fatalf("RunPlanner failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if result.Phase != "planning" {
		t.Errorf("expected phase 'planning', got %s", result.Phase)
	}
	if result.Metadata == nil {
		t.Error("expected non-nil metadata")
	}
	if result.Metadata["model"] == "" {
		t.Error("expected model in metadata")
	}
}

func TestOrchestrator_RunCoder_FullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := model.ChatResponse{
				ID:      "coder-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "coding-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "package user\n\ntype User struct {\n\tID    string\n\tName  string\n\tEmail string\n}"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 100, CompletionTokens: 60, TotalTokens: 160},
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

	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(router, registry, nil)

	result, err := orch.RunCoder(prompts.PlanUnit{
		Title:        "Create User struct",
		Description:  "Create a User struct with ID, Name, Email fields",
		Dependencies: []string{"Setup Project"},
	})
	if err != nil {
		t.Fatalf("RunCoder failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if result.Phase != "coding" {
		t.Errorf("expected phase 'coding', got %s", result.Phase)
	}
}

func TestOrchestrator_RunTester_FullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := model.ChatResponse{
				ID:      "tester-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "testing-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "## Summary\nAll tests passed successfully.\n\n## Coverage\nTest coverage: 85%"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 80, CompletionTokens: 40, TotalTokens: 120},
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
		Temperature: 0.7,
		MaxTokens:   4096,
	})

	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(router, registry, nil)

	result, err := orch.RunTester(
		"package user\n\ntype User struct {\n\tID string\n}",
		"=== RUN TestUserCreation\n--- PASS: TestUserCreation (0.00s)",
	)
	if err != nil {
		t.Fatalf("RunTester failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if result.Phase != "testing" {
		t.Errorf("expected phase 'testing', got %s", result.Phase)
	}
	if result.Metadata == nil {
		t.Error("expected non-nil metadata")
	}
	if result.Metadata["passed"] != "true" {
		t.Errorf("expected passed=true, got %s", result.Metadata["passed"])
	}

	// Verify JSON content
	var testReport TestReport
	if err := json.Unmarshal([]byte(result.Output), &testReport); err != nil {
		t.Fatalf("failed to unmarshal test report: %v", err)
	}
	if !testReport.Passed {
		t.Error("expected passed=true")
	}
}

func TestOrchestrator_RunReviewer_FullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := model.ChatResponse{
				ID:      "reviewer-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "review-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "## Summary\nCode is well structured.\n\n## Score: 90\n\n## Recommendation\nApprove"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 70, CompletionTokens: 30, TotalTokens: 100},
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
	orch := NewOrchestrator(router, registry, nil)

	result, err := orch.RunReviewer(
		"package user\n\ntype User struct {\n\tID string\n}",
		"1. Create User struct with ID field\n2. Add validation",
	)
	if err != nil {
		t.Fatalf("RunReviewer failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if result.Phase != "review" {
		t.Errorf("expected phase 'review', got %s", result.Phase)
	}
	if result.Metadata == nil {
		t.Error("expected non-nil metadata")
	}
	if result.Metadata["score"] != "90" {
		t.Errorf("expected score=90, got %s", result.Metadata["score"])
	}
	if result.Metadata["recommendation"] != "Approve" {
		t.Errorf("expected recommendation=Approve, got %s", result.Metadata["recommendation"])
	}

	// Verify JSON content
	var reviewReport ReviewReport
	if err := json.Unmarshal([]byte(result.Output), &reviewReport); err != nil {
		t.Fatalf("failed to unmarshal review report: %v", err)
	}
	if reviewReport.Score != 90 {
		t.Errorf("expected score=90, got %d", reviewReport.Score)
	}
	if reviewReport.Recommendation != "Approve" {
		t.Errorf("expected recommendation=Approve, got %s", reviewReport.Recommendation)
	}
}

func TestOrchestrator_RunCoder_WithDependencies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := model.ChatResponse{
				ID:      "coder-2",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "coding-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "package user\n\ntype User struct {\n\tID    string\n\tName  string\n\tEmail string\n}"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 120, CompletionTokens: 70, TotalTokens: 190},
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

	registry := skills.NewSkillsRegistry()
	orch := NewOrchestrator(router, registry, nil)

	result, err := orch.RunCoder(prompts.PlanUnit{
		Title:        "Create User struct",
		Description:  "Create User struct",
		Dependencies: []string{"Setup", "Config"},
	})
	if err != nil {
		t.Fatalf("RunCoder failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestTestReportToAgentResult(t *testing.T) {
	report := &TestReport{
		Passed:      true,
		Failures:    []string{},
		Suggestions: []string{"Add more tests"},
		Coverage:    "85%",
		Summary:     "All tests passed",
	}

	result := testReportToAgentResult(report)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if result.Phase != "testing" {
		t.Errorf("expected phase 'testing', got %s", result.Phase)
	}
	if result.Metadata["passed"] != "true" {
		t.Errorf("expected passed=true, got %s", result.Metadata["passed"])
	}
	if result.Metadata["summary"] != "All tests passed" {
		t.Errorf("expected summary='All tests passed', got %s", result.Metadata["summary"])
	}

	// Verify JSON marshaling
	var decoded TestReport
	if err := json.Unmarshal([]byte(result.Output), &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if decoded.Passed != report.Passed {
		t.Errorf("expected Passed=%t, got %t", report.Passed, decoded.Passed)
	}
	if decoded.Coverage != report.Coverage {
		t.Errorf("expected Coverage=%q, got %q", report.Coverage, decoded.Coverage)
	}
}

func TestReviewReportToAgentResult(t *testing.T) {
	report := &ReviewReport{
		Issues:         []string{"Missing error handling"},
		Suggestions:    []string{"Add error wrapping"},
		Score:          75,
		Recommendation: "Approve with fixes",
		Summary:        "Good code with minor issues",
	}

	result := reviewReportToAgentResult(report)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if result.Phase != "review" {
		t.Errorf("expected phase 'review', got %s", result.Phase)
	}
	if result.Metadata["score"] != "75" {
		t.Errorf("expected score=75, got %s", result.Metadata["score"])
	}
	if result.Metadata["recommendation"] != "Approve with fixes" {
		t.Errorf("expected recommendation='Approve with fixes', got %s", result.Metadata["recommendation"])
	}

	// Verify JSON marshaling
	var decoded ReviewReport
	if err := json.Unmarshal([]byte(result.Output), &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if decoded.Score != report.Score {
		t.Errorf("expected Score=%d, got %d", report.Score, decoded.Score)
	}
	if decoded.Recommendation != report.Recommendation {
		t.Errorf("expected Recommendation=%q, got %q", report.Recommendation, decoded.Recommendation)
	}
}
