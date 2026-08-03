package prompts

import (
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/model"
)

// BuildReviewerPromptFromRequest constructs the full chat message list for the reviewer agent
// from a user request and generated code.
func BuildReviewerPromptFromRequest(code string, userRequest string, skills []string) ([]model.ChatMessage, error) {
	if code == "" {
		return nil, fmt.Errorf("reviewer prompt: code is required")
	}
	if userRequest == "" {
		return nil, fmt.Errorf("reviewer prompt: user request is required")
	}
	systemMessage := buildReviewerSystemMessage(skills)
	userMessage := buildReviewerUserMessageFromRequest(code, userRequest)
	return []model.ChatMessage{
		{Role: "system", Content: systemMessage},
		{Role: "user", Content: userMessage},
	}, nil
}

// BuildReviewerPrompt constructs the full chat message list for the reviewer agent.
// It combines system-level instructions (role, skills, review checklist)
// with user-level content (code, plan/spec, previous feedback).
// DEPRECATED: Use BuildReviewerPromptFromRequest instead.
func BuildReviewerPrompt(code string, plan string, skills []string) ([]model.ChatMessage, error) {
	if code == "" {
		return nil, fmt.Errorf("reviewer prompt: code is required")
	}
	if plan == "" {
		return nil, fmt.Errorf("reviewer prompt: plan/spec is required")
	}

	systemMessage := buildReviewerSystemMessage(skills)
	userMessage := buildReviewerUserMessage(code, plan)

	return []model.ChatMessage{
		{Role: "system", Content: systemMessage},
		{Role: "user", Content: userMessage},
	}, nil
}

// buildReviewerSystemMessage constructs the system prompt with role, skill templates, review checklist, and constraints.
func buildReviewerSystemMessage(skills []string) string {
	var sb promptBuilder

	// Role definition
	sb.AppendLine("You are an expert code reviewer and quality assurance specialist. Your job is to review code against the original plan/spec, identify issues, suggest improvements, and provide a quality score.")

	// Skill templates
	if len(skills) > 0 {
		sb.AppendLine("")
		sb.AppendLine("## Available Skills")
		sb.AppendLine("You have access to the following review skills. Use them to guide your analysis:")
		sb.AppendLine("")
		for _, skill := range skills {
			sb.AppendLine(fmt.Sprintf("- **%s**", skill))
		}
	}

	// Output format
	sb.AppendLine("")
	sb.AppendLine("## Output Format")
	sb.AppendLine("Return your review in the following structured format:")
	sb.AppendLine("")
	sb.AppendLine("## Summary")
	sb.AppendLine("Brief overview of the code quality and alignment with the plan.")
	sb.AppendLine("")
	sb.AppendLine("## Issues")
	sb.AppendLine("- List each issue found with its severity (Critical, Major, Minor)")
	sb.AppendLine("- Include the specific code location and what needs to be fixed")
	sb.AppendLine("")
	sb.AppendLine("## Suggestions")
	sb.AppendLine("- Provide actionable recommendations to improve code quality")
	sb.AppendLine("- Suggest refactoring opportunities")
	sb.AppendLine("- Recommend improvements for readability and maintainability")
	sb.AppendLine("")
	sb.AppendLine("## Score")
	sb.AppendLine("Provide a quality score from 0-100 based on your analysis.")
	sb.AppendLine("")
	sb.AppendLine("## Recommendation")
	sb.AppendLine("Provide a clear recommendation: Approve, Request Changes, or Reject.")

	// Review checklist
	sb.AppendLine("")
	sb.AppendLine("## Review Checklist")
	sb.AppendLine("When reviewing the code, check against these criteria:")
	sb.AppendLine("")
	sb.AppendLine("1. **Plan Alignment**")
	sb.AppendLine("   - Does the implementation match the original plan/spec?")
	sb.AppendLine("   - Are all planned features implemented?")
	sb.AppendLine("   - Are there any deviations from the spec?")
	sb.AppendLine("")
	sb.AppendLine("2. **Code Quality**")
	sb.AppendLine("   - Follows SOLID principles")
	sb.AppendLine("   - Adheres to DRY (no code duplication)")
	sb.AppendLine("   - Follows KISS (keep it simple)")
	sb.AppendLine("   - Proper error handling")
	sb.AppendLine("   - Meaningful variable and function names")
	sb.AppendLine("")
	sb.AppendLine("3. **Security**")
	sb.AppendLine("   - No hardcoded secrets or credentials")
	sb.AppendLine("   - Input validation and sanitization")
	sb.AppendLine("   - Proper authentication/authorization checks")
	sb.AppendLine("   - No SQL injection or XSS vulnerabilities")
	sb.AppendLine("")
	sb.AppendLine("4. **Performance**")
	sb.AppendLine("   - Efficient algorithms and data structures")
	sb.AppendLine("   - No unnecessary allocations or copies")
	sb.AppendLine("   - Proper use of concurrency primitives")
	sb.AppendLine("   - No memory leaks or resource leaks")
	sb.AppendLine("")
	sb.AppendLine("5. **Testing Readiness**")
	sb.AppendLine("   - Code is testable (injectable dependencies)")
	sb.AppendLine("   - Clear separation of concerns")
	sb.AppendLine("   - No hardcoded values that prevent testing")
	sb.AppendLine("   - Proper error returns for testability")
	sb.AppendLine("")
	sb.AppendLine("6. **Documentation**")
	sb.AppendLine("   - Public functions have godoc comments")
	sb.AppendLine("   - Complex logic has inline comments")
	sb.AppendLine("   - README or documentation exists if needed")

	// Constraints
	sb.AppendLine("")
	sb.AppendLine("## Constraints")
	sb.AppendLine("- Be specific and actionable in your feedback")
	sb.AppendLine("- Prioritize issues by severity (Critical > Major > Minor)")
	sb.AppendLine("- Reference specific code locations when possible")
	sb.AppendLine("- Maintain objective, professional tone")
	sb.AppendLine("- Do NOT rewrite the code — only review and suggest")
	sb.AppendLine("- Consider both functional correctness and code quality")
	sb.AppendLine("- Follow Go best practices (idiomatic Go, standard library usage)")

	return sb.String()
}

// buildReviewerUserMessageFromRequest constructs the user prompt from code and user request.
func buildReviewerUserMessageFromRequest(code string, userRequest string) string {
	var sb promptBuilder
	sb.AppendLine("## User Request")
	sb.AppendLine(userRequest)
	sb.AppendLine("")
	sb.AppendLine("## Generated Code")
	sb.AppendLine("Review this code against the user's request above:")
	sb.AppendLine("```go")
	sb.AppendLine(code)
	sb.AppendLine("```")
	sb.AppendLine("")
	sb.AppendLine("## Review Criteria")
	sb.AppendLine("1. Does the code fulfill the user's request?")
	sb.AppendLine("2. Is the code clean, idiomatic, and well-structured?")
	sb.AppendLine("3. Are there any bugs or logical errors?")
	sb.AppendLine("4. Is error handling appropriate?")
	sb.AppendLine("5. Are there any security concerns?")
	sb.AppendLine("")
	sb.AppendLine("Provide your review in the structured format specified in the system prompt.")
	return sb.String()
}

// buildReviewerUserMessage constructs the user prompt with code, plan/spec, and previous feedback.
// DEPRECATED: Use buildReviewerUserMessageFromRequest instead.
func buildReviewerUserMessage(code string, plan string) string {
	var sb promptBuilder

	sb.AppendLine("## Code to Review")
	sb.AppendLine("```go")
	sb.AppendLine(code)
	sb.AppendLine("```")

	sb.AppendLine("")
	sb.AppendLine("## Original Plan/Spec")
	sb.AppendLine("```")
	sb.AppendLine(plan)
	sb.AppendLine("```")

	sb.AppendLine("")
	sb.AppendLine("Review the code against the plan/spec above. Identify issues, suggest improvements, and provide a quality score and recommendation.")

	return sb.String()
}
