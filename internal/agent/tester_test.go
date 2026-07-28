package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/internal/agent/skills"
	"github.com/nanaki-93/mini-orca/internal/model"
)

func TestNewTesterAgent(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()

	agent := NewTesterAgent(router, registry)
	if agent == nil {
		t.Fatal("expected non-nil tester agent")
	}
	if agent.Name() != "tester" {
		t.Errorf("expected name 'tester', got %s", agent.Name())
	}
	if agent.Description() != "Tests code and validates functionality against requirements" {
		t.Errorf("unexpected description: %s", agent.Description())
	}
	if agent.Phase() != model.PhaseTesting {
		t.Errorf("expected phase %s, got %s", model.PhaseTesting, agent.Phase())
	}
}

func TestTesterAgent_Execute_NoRouter(t *testing.T) {
	registry := skills.NewSkillsRegistry()
	agent := NewTesterAgent(nil, registry)

	_, err := agent.Execute(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for nil router")
	}
}

func TestTesterAgent_Execute_EmptyInput(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	agent := NewTesterAgent(router, registry)

	_, err := agent.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestTesterAgent_Execute_NoRegistry(t *testing.T) {
	router := model.NewRouter()
	agent := NewTesterAgent(router, nil)

	_, err := agent.Execute(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

func TestTesterAgent_Execute_FullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req model.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			resp := model.ChatResponse{
				ID:      "tester-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "testing-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "## Summary\nAll tests passed successfully.\n\n## Coverage: 95%"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 80, CompletionTokens: 50, TotalTokens: 130},
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

	result, err := agent.Execute(context.Background(), "package user\n\nfunc TestUser(t *testing.T) {\n\t// test code\n}")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil report")
	}
	if result.Passed != true {
		t.Error("expected tests to pass")
	}
	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if result.Coverage != "95%" {
		t.Errorf("expected coverage '95%%', got %s", result.Coverage)
	}
}

func TestTesterAgent_Execute_WithFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := model.ChatResponse{
				ID:      "tester-2",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "testing-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "## Summary\nTests failed.\n\n## Failures\n- TestUser_Create fails with nil pointer\n- TestUser_Update returns 500 error\n\n## Suggestions\n- Add nil checks for user creation\n- Handle database errors properly"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 90, CompletionTokens: 70, TotalTokens: 160},
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

	result, err := agent.Execute(context.Background(), "test code with failures")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Passed != false {
		t.Error("expected tests to fail")
	}
	if len(result.Failures) < 2 {
		t.Errorf("expected at least 2 failures, got %d", len(result.Failures))
	}
	if len(result.Suggestions) < 2 {
		t.Errorf("expected at least 2 suggestions, got %d", len(result.Suggestions))
	}
}

func TestTesterAgent_Execute_WithSkills(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := model.ChatResponse{
				ID:      "tester-3",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "testing-model",
				Choices: []model.ChatChoice{
					{
						Index:        0,
						Message:      model.ChatMessage{Role: "assistant", Content: "## Summary\nAll tests passed.\n\n## Coverage: 88%"},
						FinishReason: "stop",
					},
				},
				Usage: model.ChatUsage{PromptTokens: 70, CompletionTokens: 40, TotalTokens: 110},
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
	agent.SetSkills([]string{"test_automation", "edge_cases", "performance_testing"})

	result, err := agent.Execute(context.Background(), "test input with skills")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil report")
	}
	if result.Passed != true {
		t.Error("expected tests to pass")
	}
}

func TestTesterAgent_GetSkills(t *testing.T) {
	router := model.NewRouter()
	registry := skills.NewSkillsRegistry()
	agent := NewTesterAgent(router, registry)

	// Default skills should be empty
	skills := agent.GetSkills()
	if skills == nil {
		t.Fatal("expected non-nil skills slice")
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}

	// Set skills
	agent.SetSkills([]string{"test_automation", "edge_cases"})
	skills = agent.GetSkills()
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}

func TestParseTestReport_Pass(t *testing.T) {
	content := "## Summary\nAll tests passed successfully.\n\n## Coverage: 95%"
	report := parseTestReport(content)

	if report.Passed != true {
		t.Error("expected passed=true")
	}
	if report.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if report.Coverage != "95%" {
		t.Errorf("expected coverage '95%%', got %s", report.Coverage)
	}
}

func TestParseTestReport_Fail(t *testing.T) {
	content := "## Summary\nTests failed with errors.\n\n## Failures\n- TestA failed\n- TestB failed\n\n## Suggestions\n- Fix TestA\n- Fix TestB"
	report := parseTestReport(content)

	if report.Passed != false {
		t.Error("expected passed=false")
	}
	if len(report.Failures) < 2 {
		t.Errorf("expected at least 2 failures, got %d", len(report.Failures))
	}
	if len(report.Suggestions) < 2 {
		t.Errorf("expected at least 2 suggestions, got %d", len(report.Suggestions))
	}
}

func TestParseTestReport_NoCoverage(t *testing.T) {
	content := "## Summary\nAll tests passed."
	report := parseTestReport(content)

	if report.Coverage != "" {
		t.Errorf("expected empty coverage, got %s", report.Coverage)
	}
}

func TestParseTestReport_NoFailures(t *testing.T) {
	content := "## Summary\nAll tests passed.\n\n## Coverage: 80%"
	report := parseTestReport(content)

	if len(report.Failures) != 0 {
		t.Errorf("expected 0 failures, got %d", len(report.Failures))
	}
}

func TestExtractCoverage(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "coverage with percentage",
			content:  "Test coverage: 85%",
			expected: "85%",
		},
		{
			name:     "coverage without percentage",
			content:  "The coverage is 92.5",
			expected: "92.5",
		},
		{
			name:     "no coverage mentioned",
			content:  "All tests passed.",
			expected: "",
		},
		{
			name:     "coverage in middle of text",
			content:  "Summary\nCoverage: 78%\nMore text",
			expected: "78%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCoverage(tt.content)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		header   string
		expected []string
	}{
		{
			name:     "extract failures",
			content:  "## Failures\n- TestA failed\n- TestB failed\n",
			header:   "## Failures",
			expected: []string{"TestA failed", "TestB failed"},
		},
		{
			name:     "extract suggestions",
			content:  "## Suggestions\n- Add error handling\n- Improve logging\n",
			header:   "## Suggestions",
			expected: []string{"Add error handling", "Improve logging"},
		},
		{
			name:     "header not found",
			content:  "Some content without header",
			header:   "## NonExistent",
			expected: nil,
		},
		{
			name:     "no bullet points",
			content:  "## Failures\nNo failures here",
			header:   "## Failures",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLines(tt.content, tt.header)
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

func TestTestReport_Struct(t *testing.T) {
	report := &TestReport{
		Passed:      true,
		Failures:    []string{"TestA", "TestB"},
		Suggestions: []string{"Fix A", "Fix B"},
		Coverage:    "90%",
		Summary:     "All tests passed",
	}

	// Verify JSON marshaling works
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal report: %v", err)
	}

	// Verify JSON unmarshaling works
	var decoded TestReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal report: %v", err)
	}

	if decoded.Passed != report.Passed {
		t.Errorf("expected Passed=%v, got %v", report.Passed, decoded.Passed)
	}
	if len(decoded.Failures) != len(report.Failures) {
		t.Errorf("expected %d failures, got %d", len(report.Failures), len(decoded.Failures))
	}
	if len(decoded.Suggestions) != len(report.Suggestions) {
		t.Errorf("expected %d suggestions, got %d", len(report.Suggestions), len(decoded.Suggestions))
	}
	if decoded.Coverage != report.Coverage {
		t.Errorf("expected coverage %q, got %q", report.Coverage, decoded.Coverage)
	}
	if decoded.Summary != report.Summary {
		t.Errorf("expected summary %q, got %q", report.Summary, decoded.Summary)
	}
}

func TestTesterAgent_Execute_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := model.ChatResponse{
				ID:      "tester-4",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "testing-model",
				Choices: []model.ChatChoice{},
				Usage:   model.ChatUsage{PromptTokens: 10, CompletionTokens: 0, TotalTokens: 10},
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

	_, err := agent.Execute(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected 'empty response' in error, got: %v", err)
	}
}
