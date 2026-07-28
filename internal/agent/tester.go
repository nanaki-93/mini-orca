package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/internal/agent/skills"
	"github.com/nanaki-93/mini-orca/internal/model"
)

// TestReport represents the output of a testing phase.
type TestReport struct {
	Passed      bool     `json:"passed"`
	Failures    []string `json:"failures,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	Coverage    string   `json:"coverage,omitempty"`
	Summary     string   `json:"summary"`
}

// TesterAgent is the agent responsible for testing code.
// It embeds *Client to inherit Agent interface implementation.
type TesterAgent struct {
	*Client
	registry *skills.SkillsRegistry
}

// NewTesterAgent creates a new tester agent with the given router and skills registry.
func NewTesterAgent(router *model.Router, registry *skills.SkillsRegistry) *TesterAgent {
	client := NewClient(router)
	client.name = "tester"
	client.description = "Tests code and validates functionality against requirements"
	client.phase = model.PhaseTesting
	client.skills = make([]string, 0)

	return &TesterAgent{
		Client:   client,
		registry: registry,
	}
}

// Execute runs the tester agent with the given code and test results.
// It builds a system prompt from tester skills, calls the LLM, and returns a test report.
func (t *TesterAgent) Execute(ctx context.Context, input string) (*TestReport, error) {
	if t.router == nil {
		return nil, fmt.Errorf("tester agent: router not configured")
	}

	if input == "" {
		return nil, fmt.Errorf("tester agent: input is required")
	}

	if t.registry == nil {
		return nil, fmt.Errorf("tester agent: skills registry not configured")
	}

	// Build system prompt from tester skills
	systemPrompt := t.registry.BuildSkillPrompt(t.GetSkills())
	if systemPrompt == "" {
		systemPrompt = "You are an expert QA tester. Analyze code and test results to identify failures and suggest improvements."
	}

	// Combine system prompt with user input (code + test results)
	fullPrompt := systemPrompt + "\n\n---\n\nCode and Test Results:\n" + input

	// Call the LLM with testing phase config
	messages := []model.ChatMessage{
		{Role: "user", Content: fullPrompt},
	}

	resp, err := t.router.Chat(string(t.phase), messages)
	if err != nil {
		return nil, fmt.Errorf("tester agent: execution failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("tester agent: empty response from model")
	}

	// Parse LLM response into TestReport
	report := parseTestReport(resp.Choices[0].Message.Content)

	return report, nil
}

// parseTestReport parses the LLM response into a TestReport struct.
func parseTestReport(content string) *TestReport {
	report := &TestReport{
		Summary: content,
	}

	// Determine pass/fail status
	lowerContent := strings.ToLower(content)
	report.Passed = !strings.Contains(lowerContent, "fail") && !strings.Contains(lowerContent, "error") && !strings.Contains(lowerContent, "bug")

	// Extract failures
	report.Failures = extractLines(content, "## Failures")
	if len(report.Failures) == 0 {
		report.Failures = extractLines(content, "## Issues")
	}

	// Extract suggestions
	report.Suggestions = extractLines(content, "## Suggestions")
	if len(report.Suggestions) == 0 {
		report.Suggestions = extractLines(content, "## Recommendations")
	}

	// Extract coverage
	report.Coverage = extractCoverage(content)

	return report
}

// extractLines extracts bullet-pointed lines after a section header.
func extractLines(content, header string) []string {
	var lines []string
	// Find the header
	headerIdx := strings.Index(content, header)
	if headerIdx == -1 {
		return nil
	}

	// Get content after header
	afterHeader := content[headerIdx+len(header):]

	// Split by newlines and extract bullet points
	for _, line := range strings.Split(afterHeader, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			lines = append(lines, strings.TrimPrefix(line, "- "))
		} else if strings.HasPrefix(line, "* ") {
			lines = append(lines, strings.TrimPrefix(line, "* "))
		}
	}

	return lines
}

// extractCoverage extracts coverage percentage from content.
func extractCoverage(content string) string {
	// Look for common coverage patterns
	for _, pattern := range []string{"coverage:", "coverage:", "coverage is", "test coverage"} {
		idx := strings.Index(strings.ToLower(content), pattern)
		if idx != -1 {
			// Extract next few characters after pattern
			after := content[idx+len(pattern):]
			var coverage string
			for _, ch := range after {
				if ch >= '0' && ch <= '9' || ch == '.' || ch == '%' {
					coverage += string(ch)
				} else if len(coverage) > 0 {
					break
				}
			}
			if coverage != "" {
				return coverage
			}
		}
	}
	return ""
}
