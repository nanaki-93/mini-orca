package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func TestNewOrchestrator(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)

	orch := NewOrchestrator(client, nil)
	if orch == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	if orch.llmClient == nil {
		t.Error("expected LLM client to be set")
	}
}

func TestRunCoderFromPrompt_EmptyPrompt(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	orch := NewOrchestrator(client, nil)

	_, err := orch.RunCoderFromPrompt("", "")
	if err == nil {
		t.Fatal("expected error for empty prompt, got nil")
	}
}

func TestRunCoderFromPrompt_Valid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := llm.ChatResponse{
				ID:      "coder-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "coding-model",
				Choices: []llm.ChatChoice{
					{
						Index:        0,
						Message:      llm.ChatMessage{Role: "assistant", Content: "package user\n\ntype User struct {\n\tID    string\n\tName  string\n\tEmail string\n}"},
						FinishReason: "stop",
					},
				},
				Usage: llm.ChatUsage{PromptTokens: 100, CompletionTokens: 60, TotalTokens: 160},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test-key", "coding-model", 0.7, 4096)
	orch := NewOrchestrator(client, nil)

	result, err := orch.RunCoderFromPrompt("Create a User struct with ID, Name, Email fields", "")
	if err != nil {
		t.Fatalf("RunCoderFromPrompt failed: %v", err)
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

func TestRunCoderFromPrompt_WithProjectContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := llm.ChatResponse{
				ID:      "coder-2",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "coding-model",
				Choices: []llm.ChatChoice{
					{
						Index:        0,
						Message:      llm.ChatMessage{Role: "assistant", Content: "package user\n\ntype User struct {\n\tID string `json:\"id\"`\n}"},
						FinishReason: "stop",
					},
				},
				Usage: llm.ChatUsage{PromptTokens: 120, CompletionTokens: 70, TotalTokens: 190},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test-key", "coding-model", 0.7, 4096)
	orch := NewOrchestrator(client, nil)

	projectContext := "module github.com/example/project\ngo 1.21"
	result, err := orch.RunCoderFromPrompt("Create User struct", projectContext)
	if err != nil {
		t.Fatalf("RunCoderFromPrompt with project context failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestRunReviewer_EmptyCode(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	orch := NewOrchestrator(client, nil)

	_, err := orch.RunReviewer("", "Create User struct")
	if err == nil {
		t.Fatal("expected error for empty code, got nil")
	}
}

func TestRunReviewer_EmptyUserPrompt(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	orch := NewOrchestrator(client, nil)

	_, err := orch.RunReviewer("package user", "")
	if err == nil {
		t.Fatal("expected error for empty user prompt, got nil")
	}
}

func TestRunReviewer_Valid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := llm.ChatResponse{
				ID:      "reviewer-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "review-model",
				Choices: []llm.ChatChoice{
					{
						Index:        0,
						Message:      llm.ChatMessage{Role: "assistant", Content: "## Summary\nCode is well structured.\n\n## Score: 90\n\n## Recommendation\nApprove"},
						FinishReason: "stop",
					},
				},
				Usage: llm.ChatUsage{PromptTokens: 70, CompletionTokens: 30, TotalTokens: 100},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test-key", "review-model", 0.2, 4096)
	orch := NewOrchestrator(client, nil)

	result, err := orch.RunReviewer(
		"package user\n\ntype User struct {\n\tID string\n}",
		"Create a User struct with ID field",
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

func TestRunTester_EmptyCode(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	orch := NewOrchestrator(client, nil)

	_, err := orch.RunTester("", "test results")
	if err == nil {
		t.Fatal("expected error for empty code, got nil")
	}
}

func TestRunTester_EmptyResults(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	orch := NewOrchestrator(client, nil)

	_, err := orch.RunTester("code", "")
	if err == nil {
		t.Fatal("expected error for empty test results, got nil")
	}
}

func TestRunTester_Valid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := llm.ChatResponse{
				ID:      "tester-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "testing-model",
				Choices: []llm.ChatChoice{
					{
						Index:        0,
						Message:      llm.ChatMessage{Role: "assistant", Content: "## Summary\nAll tests passed successfully.\n\n## Coverage\nTest coverage: 85%"},
						FinishReason: "stop",
					},
				},
				Usage: llm.ChatUsage{PromptTokens: 80, CompletionTokens: 40, TotalTokens: 120},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test-key", "testing-model", 0.7, 4096)
	orch := NewOrchestrator(client, nil)

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
