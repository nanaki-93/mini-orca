package prompts

import (
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

// PlanUnit represents an atomic unit of work to be implemented by the coder.
type PlanUnit struct {
	Title        string   // Clear, concise title of the unit
	Description  string   // Detailed description of what needs to be done
	Dependencies []string // List of other unit titles this depends on
}

// FeatureRequest represents a user's feature request for the coder agent.
type FeatureRequest struct {
	Prompt         string
	ProjectContext string
	TargetFile     string
	Language       string
}

// BuildCoderPromptFromRequest constructs the full chat message list for the coder agent
// from a FeatureRequest.
func BuildCoderPromptFromRequest(req FeatureRequest) ([]llm.ChatMessage, error) {
	if req.Prompt == "" {
		return nil, fmt.Errorf("coder prompt: feature request prompt is required")
	}
	systemMessage := buildCoderSystemMessage(nil)
	userMessage := buildCoderUserMessageFromRequest(req)
	return []llm.ChatMessage{
		{Role: "system", Content: systemMessage},
		{Role: "user", Content: userMessage},
	}, nil
}

// buildCoderSystemMessage constructs the system prompt with role, skill templates, output format, and constraints.
func buildCoderSystemMessage(skills []string) string {
	var sb promptBuilder

	// Role definition
	sb.AppendLine("You are an expert Go coder. Your job is to implement clean, idiomatic, well-tested Go code for atomic units of work.")

	// Skill templates
	if len(skills) > 0 {
		sb.AppendLine("")
		sb.AppendLine("## Available Skills")
		sb.AppendLine("You have access to the following coding skills. Use them to guide your implementation:")
		sb.AppendLine("")
		for _, skill := range skills {
			sb.AppendLine(fmt.Sprintf("- **%s**", skill))
		}
	}

	// Output format
	sb.AppendLine("")
	sb.AppendLine("## Output Format")
	sb.AppendLine("Return ONLY the Go code for this atomic unit. Follow this structure:")
	sb.AppendLine("")
	sb.AppendLine("1. Package declaration")
	sb.AppendLine("2. Imports (standard library first, then external)")
	sb.AppendLine("3. Types, interfaces, and implementations")
	sb.AppendLine("4. Error handling (always explicit)")
	sb.AppendLine("")
	sb.AppendLine("Code must be:")
	sb.AppendLine("- Self-contained (no external dependencies unless specified)")
	sb.AppendLine("- Properly formatted with gofmt standards")
	sb.AppendLine("- Include meaningful comments for public identifiers")
	sb.AppendLine("- Follow Go naming conventions (camelCase for unexported, PascalCase for exported)")

	// Constraints
	sb.AppendLine("")
	sb.AppendLine("## Constraints")
	sb.AppendLine("- Apply SOLID principles (Single Responsibility, Open/Closed, Liskov Substitution, Interface Segregation, Dependency Inversion)")
	sb.AppendLine("- Follow DRY — avoid code duplication")
	sb.AppendLine("- Keep functions small and focused (one responsibility per function)")
	sb.AppendLine("- Always handle errors explicitly (no silent failures)")
	sb.AppendLine("- Use context.Context for operations that may be cancelled or timed out")
	sb.AppendLine("- Follow the KISS principle — prefer simple solutions")
	sb.AppendLine("- Write code that is easy to test (inject dependencies, avoid globals)")
	sb.AppendLine("- Do NOT include tests in this output (testing is a separate phase)")

	return sb.String()
}

// buildCoderUserMessageFromRequest constructs the user prompt from a FeatureRequest.
func buildCoderUserMessageFromRequest(req FeatureRequest) string {
	var sb promptBuilder
	sb.AppendLine("## Feature Request")
	sb.AppendLine(req.Prompt)
	sb.AppendLine("")
	if req.TargetFile != "" {
		sb.AppendLine("## Target File")
		sb.AppendLine(fmt.Sprintf("Write the code to: %s", req.TargetFile))
		sb.AppendLine("")
	}
	if req.ProjectContext != "" {
		sb.AppendLine("## Project Context")
		sb.AppendLine("Reference this existing project context:")
		sb.AppendLine("```")
		sb.AppendLine(req.ProjectContext)
		sb.AppendLine("```")
		sb.AppendLine("")
	}
	sb.AppendLine("## Instructions")
	sb.AppendLine("1. Generate clean, idiomatic code for this feature")
	sb.AppendLine("2. Include a comment at the very top: // target: path/to/target_file.go")
	sb.AppendLine("3. Return ONLY the code in a code block")
	sb.AppendLine("4. Do NOT include tests — testing is a separate phase")
	return sb.String()
}
