package prompts

import (
	"fmt"

	"github.com/nanaki-93/mini-orca/internal/model"
)

// PlanUnit represents an atomic unit of work to be implemented by the coder.
type PlanUnit struct {
	Title        string   // Clear, concise title of the unit
	Description  string   // Detailed description of what needs to be done
	Dependencies []string // List of other unit titles this depends on
}

// BuildCoderPrompt constructs the full chat message list for the coder agent.
// It combines system-level instructions (role, skills, output format, constraints)
// with user-level content (unit description, existing code, dependencies).
func BuildCoderPrompt(unit PlanUnit, existingCode string, skills []string) ([]model.ChatMessage, error) {
	if unit.Title == "" {
		return nil, fmt.Errorf("coder prompt: unit title is required")
	}
	if unit.Description == "" {
		return nil, fmt.Errorf("coder prompt: unit description is required")
	}

	systemMessage := buildCoderSystemMessage(skills)
	userMessage := buildCoderUserMessage(unit, existingCode)

	return []model.ChatMessage{
		{Role: "system", Content: systemMessage},
		{Role: "user", Content: userMessage},
	}, nil
}

// buildCoderSystemMessage constructs the system prompt with role, skill templates, output format, and constraints.
func buildCoderSystemMessage(skills []string) string {
	var sb coderPromptBuilder

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

// buildCoderUserMessage constructs the user prompt with unit description, existing code, and dependencies.
func buildCoderUserMessage(unit PlanUnit, existingCode string) string {
	var sb coderPromptBuilder

	sb.AppendLine("## Atomic Unit")
	sb.AppendLine(fmt.Sprintf("Title: %s", unit.Title))
	sb.AppendLine("")
	sb.AppendLine(unit.Description)

	// Dependencies
	if len(unit.Dependencies) > 0 {
		sb.AppendLine("")
		sb.AppendLine("## Dependencies")
		sb.AppendLine("This unit depends on the following units:")
		for _, dep := range unit.Dependencies {
			sb.AppendLine(fmt.Sprintf("- %s", dep))
		}
	}

	// Existing code
	if existingCode != "" {
		sb.AppendLine("")
		sb.AppendLine("## Existing Code")
		sb.AppendLine("Reference this existing code for context:")
		sb.AppendLine("```go")
		sb.AppendLine(existingCode)
		sb.AppendLine("```")
	}

	sb.AppendLine("")
	sb.AppendLine("Implement the code for this atomic unit based on the description above.")

	return sb.String()
}

// coderPromptBuilder is a simple helper for building multi-line prompt strings.
type coderPromptBuilder struct {
	lines []string
}

// AppendLine adds a line to the coder prompt builder.
func (b *coderPromptBuilder) AppendLine(line string) {
	b.lines = append(b.lines, line)
}

// String returns the built prompt as a single string.
func (b *coderPromptBuilder) String() string {
	return joinLines(b.lines)
}
