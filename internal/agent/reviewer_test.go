package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func TestNewReviewerAgent(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)

	agent := NewReviewerAgent(client)
	if agent == nil {
		t.Fatal("expected non-nil reviewer agent")
	}
	if agent.Name() != "reviewer" {
		t.Errorf("expected name 'reviewer', got %s", agent.Name())
	}
	if agent.Description() != "Reviews code for quality, security, and adherence to standards" {
		t.Errorf("unexpected description: %s", agent.Description())
	}
	if agent.Phase() != "review" {
		t.Errorf("expected phase 'review', got %s", agent.Phase())
	}
}

func TestReviewerAgent_Execute_NoClient(t *testing.T) {
	agent := NewReviewerAgent(nil)

	_, err := agent.Execute(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for nil LLM client")
	}
}

func TestReviewerAgent_Execute_EmptyInput(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	agent := NewReviewerAgent(client)

	_, err := agent.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestReviewerAgent_Execute_FullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req llm.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			resp := llm.ChatResponse{
				ID:      "reviewer-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "review-model",
				Choices: []llm.ChatChoice{
					{
						Index:        0,
						Message:      llm.ChatMessage{Role: "assistant", Content: "## Summary\nCode quality is good with minor improvements needed.\n\n## Score: 85\n\n## Recommendation\nApprove with minor fixes"},
						FinishReason: "stop",
					},
				},
				Usage: llm.ChatUsage{PromptTokens: 80, CompletionTokens: 50, TotalTokens: 130},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test-key", "review-model", 0.2, 4096)
	agent := NewReviewerAgent(client)

	result, err := agent.Execute(context.Background(), "package user\n\nfunc NewUser(id, name string) *User {\n\treturn &User{ID: id, Name: name}\n}")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil report")
	}
	if result.Score != 85 {
		t.Errorf("expected score 85, got %d", result.Score)
	}
	if result.Recommendation != "Approve with minor fixes" {
		t.Errorf("expected recommendation 'Approve with minor fixes', got %s", result.Recommendation)
	}
	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestReviewerAgent_Execute_WithIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := llm.ChatResponse{
				ID:      "reviewer-2",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "review-model",
				Choices: []llm.ChatChoice{
					{
						Index:        0,
						Message:      llm.ChatMessage{Role: "assistant", Content: "## Summary\nMultiple issues found requiring attention.\n\n## Score: 45\n\n## Issues\n- No error handling in database operations\n- Missing input validation\n- Inconsistent naming conventions\n\n## Suggestions\n- Add error wrapping for database calls\n- Implement input validation middleware\n- Standardize naming to match project conventions\n\n## Recommendation\nReject - major refactoring required"},
						FinishReason: "stop",
					},
				},
				Usage: llm.ChatUsage{PromptTokens: 90, CompletionTokens: 70, TotalTokens: 160},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test-key", "review-model", 0.2, 4096)
	agent := NewReviewerAgent(client)

	result, err := agent.Execute(context.Background(), "test code with issues")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Score != 45 {
		t.Errorf("expected score 45, got %d", result.Score)
	}
	if len(result.Issues) < 3 {
		t.Errorf("expected at least 3 issues, got %d", len(result.Issues))
	}
	if len(result.Suggestions) < 3 {
		t.Errorf("expected at least 3 suggestions, got %d", len(result.Suggestions))
	}
	if result.Recommendation != "Reject - major refactoring required" {
		t.Errorf("expected rejection recommendation, got %s", result.Recommendation)
	}
}

func TestReviewerAgent_Execute_WithSkills(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := llm.ChatResponse{
				ID:      "reviewer-3",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "review-model",
				Choices: []llm.ChatChoice{
					{
						Index:        0,
						Message:      llm.ChatMessage{Role: "assistant", Content: "## Summary\nCode follows best practices.\n\n## Score: 92\n\n## Recommendation\nApprove"},
						FinishReason: "stop",
					},
				},
				Usage: llm.ChatUsage{PromptTokens: 70, CompletionTokens: 40, TotalTokens: 110},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test-key", "review-model", 0.2, 4096)
	agent := NewReviewerAgent(client)

	// Set reviewer-specific skills
	agent.SetSkills([]string{"security_audit", "performance_review", "clean_code"})

	result, err := agent.Execute(context.Background(), "test input with skills")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil report")
	}
	if result.Score != 92 {
		t.Errorf("expected score 92, got %d", result.Score)
	}
}

func TestReviewerAgent_GetSkills(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	agent := NewReviewerAgent(client)

	// Default skills should be empty
	agentSkills := agent.GetSkills()
	if agentSkills == nil {
		t.Fatal("expected non-nil skills slice")
	}
	if len(agentSkills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(agentSkills))
	}

	// Set skills
	agent.SetSkills([]string{"security_audit", "performance_review"})
	agentSkills = agent.GetSkills()
	if len(agentSkills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(agentSkills))
	}
}

func TestParseReviewReport_HighScore(t *testing.T) {
	content := "## Summary\nExcellent code quality.\n\n## Score: 95\n\n## Recommendation\nApprove"
	report := parseReviewReport(content)

	if report.Score != 95 {
		t.Errorf("expected score 95, got %d", report.Score)
	}
	if report.Recommendation != "Approve" {
		t.Errorf("expected recommendation 'Approve', got %s", report.Recommendation)
	}
	if report.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestParseReviewReport_LowScore(t *testing.T) {
	content := "## Summary\nMultiple critical issues found.\n\n## Score: 30\n\n## Issues\n- SQL injection vulnerability\n- No input validation\n- Race condition in concurrent access\n\n## Suggestions\n- Use parameterized queries\n- Add validation middleware\n- Add mutex for shared state\n\n## Recommendation\nReject - security risks"
	report := parseReviewReport(content)

	if report.Score != 30 {
		t.Errorf("expected score 30, got %d", report.Score)
	}
	if len(report.Issues) < 3 {
		t.Errorf("expected at least 3 issues, got %d", len(report.Issues))
	}
	if len(report.Suggestions) < 3 {
		t.Errorf("expected at least 3 suggestions, got %d", len(report.Suggestions))
	}
	if report.Recommendation != "Reject - security risks" {
		t.Errorf("expected rejection recommendation, got %s", report.Recommendation)
	}
}

func TestParseReviewReport_NoScore(t *testing.T) {
	content := "## Summary\nCode needs improvements."
	report := parseReviewReport(content)

	if report.Score != 0 {
		t.Errorf("expected score 0 when not found, got %d", report.Score)
	}
}

func TestParseReviewReport_NoRecommendation(t *testing.T) {
	content := "## Summary\nCode needs improvements.\n\n## Score: 60"
	report := parseReviewReport(content)

	if report.Recommendation != "Review complete. See issues and suggestions above." {
		t.Errorf("expected default recommendation, got %s", report.Recommendation)
	}
}

func TestParseReviewReport_NoIssues(t *testing.T) {
	content := "## Summary\nAll good.\n\n## Score: 90\n\n## Suggestions\n- Add more comments\n- Extract helper functions"
	report := parseReviewReport(content)

	if len(report.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(report.Issues))
	}
	if len(report.Suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d", len(report.Suggestions))
	}
}

func TestExtractScore(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "score with colon",
			content:  "Score: 85",
			expected: 85,
		},
		{
			name:     "score is pattern",
			content:  "The score is 92",
			expected: 92,
		},
		{
			name:     "rating pattern",
			content:  "Rating: 78",
			expected: 78,
		},
		{
			name:     "quality score pattern",
			content:  "Quality score: 88",
			expected: 88,
		},
		{
			name:     "no score",
			content:  "All tests passed.",
			expected: 0,
		},
		{
			name:     "score in middle of text",
			content:  "Summary\nScore: 75\nMore text",
			expected: 75,
		},
		{
			name:     "invalid score",
			content:  "Score: 150",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractScore(tt.content)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestExtractRecommendation(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "recommendation section",
			content:  "## Recommendation\nApprove with minor fixes",
			expected: "Approve with minor fixes",
		},
		{
			name:     "verdict section",
			content:  "## Verdict\nReject - needs refactoring",
			expected: "Reject - needs refactoring",
		},
		{
			name:     "conclusion section",
			content:  "## Conclusion\nNeeds more work",
			expected: "Needs more work",
		},
		{
			name:     "final decision section",
			content:  "## Final Decision\nMerge after fixes",
			expected: "Merge after fixes",
		},
		{
			name:     "no recommendation",
			content:  "Summary only",
			expected: "Review complete. See issues and suggestions above.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRecommendation(tt.content)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestReviewerExtractLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		header   string
		expected []string
	}{
		{
			name:     "extract issues",
			content:  "## Issues\n- Issue A\n- Issue B\n",
			header:   "## Issues",
			expected: []string{"Issue A", "Issue B"},
		},
		{
			name:     "extract suggestions",
			content:  "## Suggestions\n- Suggestion A\n- Suggestion B\n",
			header:   "## Suggestions",
			expected: []string{"Suggestion A", "Suggestion B"},
		},
		{
			name:     "header not found",
			content:  "Some content without header",
			header:   "## NonExistent",
			expected: nil,
		},
		{
			name:     "no bullet points",
			content:  "## Issues\nNo issues here",
			header:   "## Issues",
			expected: nil,
		},
		{
			name:     "star bullet points",
			content:  "## Suggestions\n* Suggestion A\n* Suggestion B\n",
			header:   "## Suggestions",
			expected: []string{"Suggestion A", "Suggestion B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reviewerExtractLines(tt.content, tt.header)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d", len(tt.expected), len(result))
			}
			for i, line := range result {
				if i < len(tt.expected) && line != tt.expected[i] {
					t.Errorf("expected line %d to be %q, got %q", i, tt.expected[i], line)
				}
			}
		})
	}
}

func TestReviewReport_Struct(t *testing.T) {
	report := &ReviewReport{
		Issues:         []string{"Issue1", "Issue2"},
		Suggestions:    []string{"Suggestion1", "Suggestion2"},
		Score:          75,
		Recommendation: "Approve with fixes",
		Summary:        "Code review summary",
	}

	// Verify JSON marshaling works
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal report: %v", err)
	}

	// Verify JSON unmarshaling works
	var decoded ReviewReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal report: %v", err)
	}

	if decoded.Score != report.Score {
		t.Errorf("expected Score=%d, got %d", report.Score, decoded.Score)
	}
	if decoded.Recommendation != report.Recommendation {
		t.Errorf("expected recommendation %q, got %q", report.Recommendation, decoded.Recommendation)
	}
	if decoded.Summary != report.Summary {
		t.Errorf("expected summary %q, got %q", report.Summary, decoded.Summary)
	}
}

func TestReviewerAgent_Execute_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := llm.ChatResponse{
				ID:      "reviewer-4",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "review-model",
				Choices: []llm.ChatChoice{},
				Usage:   llm.ChatUsage{PromptTokens: 10, CompletionTokens: 0, TotalTokens: 10},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := llm.NewClient(server.URL, "test-key", "review-model", 0.2, 4096)
	agent := NewReviewerAgent(client)

	_, err := agent.Execute(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %v", err)
	}
}
