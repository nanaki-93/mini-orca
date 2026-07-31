package prompts

import (
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/model"
)

// BuildTesterPrompt constructs the full chat message list for the tester agent.
// It combines system-level instructions (role, skills, analysis framework)
// with user-level content (code, test output, coverage report).
func BuildTesterPrompt(code string, testResults string, skills []string) ([]model.ChatMessage, error) {
	if code == "" {
		return nil, fmt.Errorf("tester prompt: code is required")
	}
	if testResults == "" {
		return nil, fmt.Errorf("tester prompt: test results are required")
	}

	systemMessage := buildTesterSystemMessage(skills)
	userMessage := buildTesterUserMessage(code, testResults)

	return []model.ChatMessage{
		{Role: "system", Content: systemMessage},
		{Role: "user", Content: userMessage},
	}, nil
}

// buildTesterSystemMessage constructs the system prompt with role, skill templates, analysis framework, and constraints.
func buildTesterSystemMessage(skills []string) string {
	var sb testerPromptBuilder

	// Role definition
	sb.AppendLine("You are an expert QA tester and code reviewer. Your job is to analyze code, test results, and coverage reports to identify failures, suggest improvements, and ensure code quality.")

	// Skill templates
	if len(skills) > 0 {
		sb.AppendLine("")
		sb.AppendLine("## Available Skills")
		sb.AppendLine("You have access to the following testing skills. Use them to guide your analysis:")
		sb.AppendLine("")
		for _, skill := range skills {
			sb.AppendLine(fmt.Sprintf("- **%s**", skill))
		}
	}

	// Output format
	sb.AppendLine("")
	sb.AppendLine("## Output Format")
	sb.AppendLine("Return your analysis in the following structured format:")
	sb.AppendLine("")
	sb.AppendLine("## Summary")
	sb.AppendLine("Brief overview of the overall test results and code quality assessment.")
	sb.AppendLine("")
	sb.AppendLine("## Failures")
	sb.AppendLine("- List each test failure with its cause and location")
	sb.AppendLine("- Include the specific assertion or condition that failed")
	sb.AppendLine("")
	sb.AppendLine("## Suggestions")
	sb.AppendLine("- Provide actionable recommendations to fix failures")
	sb.AppendLine("- Suggest improvements for test coverage and quality")
	sb.AppendLine("- Recommend code refactoring if needed")
	sb.AppendLine("")
	sb.AppendLine("## Coverage")
	sb.AppendLine("Report the test coverage percentage if available, or estimate it.")

	// Analysis framework
	sb.AppendLine("")
	sb.AppendLine("## Analysis Framework")
	sb.AppendLine("When analyzing the code and test results, follow this framework:")
	sb.AppendLine("")
	sb.AppendLine("1. **Code Review**")
	sb.AppendLine("   - Check for edge cases and error handling")
	sb.AppendLine("   - Verify adherence to SOLID principles")
	sb.AppendLine("   - Assess code readability and maintainability")
	sb.AppendLine("   - Identify potential bugs or race conditions")
	sb.AppendLine("")
	sb.AppendLine("2. **Test Analysis**")
	sb.AppendLine("   - Examine test coverage (unit, integration, edge cases)")
	sb.AppendLine("   - Identify missing test scenarios")
	sb.AppendLine("   - Evaluate test quality and assertions")
	sb.AppendLine("   - Check for flaky or unreliable tests")
	sb.AppendLine("")
	sb.AppendLine("3. **Coverage Assessment**")
	sb.AppendLine("   - Identify untested code paths")
	sb.AppendLine("   - Highlight critical paths without tests")
	sb.AppendLine("   - Suggest high-value tests to add")
	sb.AppendLine("")
	sb.AppendLine("4. **Quality Recommendations**")
	sb.AppendLine("   - Prioritize fixes by severity")
	sb.AppendLine("   - Suggest refactoring opportunities")
	sb.AppendLine("   - Recommend testing strategies")

	// Constraints
	sb.AppendLine("")
	sb.AppendLine("## Constraints")
	sb.AppendLine("- Be specific and actionable in your suggestions")
	sb.AppendLine("- Prioritize security and reliability issues")
	sb.AppendLine("- Consider both unit and integration testing")
	sb.AppendLine("- Follow Go best practices for testing (table-driven tests, benchmarks)")
	sb.AppendLine("- Do NOT write test code — only analyze and suggest")
	sb.AppendLine("- Maintain objective, professional tone")

	return sb.String()
}

// buildTesterUserMessage constructs the user prompt with code, test results, and coverage report.
func buildTesterUserMessage(code string, testResults string) string {
	var sb testerPromptBuilder

	sb.AppendLine("## Code to Test")
	sb.AppendLine("```go")
	sb.AppendLine(code)
	sb.AppendLine("```")

	sb.AppendLine("")
	sb.AppendLine("## Test Results")
	sb.AppendLine("```")
	sb.AppendLine(testResults)
	sb.AppendLine("```")

	sb.AppendLine("")
	sb.AppendLine("Based on the code and test results above, provide a comprehensive analysis following the framework in the system instructions.")

	return sb.String()
}

// testerPromptBuilder is a simple helper for building multi-line prompt strings.
type testerPromptBuilder struct {
	lines []string
}

// AppendLine adds a line to the tester prompt builder.
func (b *testerPromptBuilder) AppendLine(line string) {
	b.lines = append(b.lines, line)
}

// String returns the built prompt as a single string.
func (b *testerPromptBuilder) String() string {
	return joinLines(b.lines)
}
