package prompts

import (
	"fmt"

	"github.com/nanaki-93/mini-orca/internal/model"
)

// BuildPlannerPrompt constructs the full chat message list for the planner agent.
// It combines system-level instructions (role, skills, output format, constraints)
// with user-level content (goal, context, constraints).
func BuildPlannerPrompt(goal string, skills []string, context string) ([]model.ChatMessage, error) {
	if goal == "" {
		return nil, fmt.Errorf("planner prompt: goal is required")
	}

	systemMessage := buildSystemMessage(skills)
	userMessage := buildUserMessage(goal, context)

	return []model.ChatMessage{
		{Role: "system", Content: systemMessage},
		{Role: "user", Content: userMessage},
	}, nil
}

// buildSystemMessage constructs the system prompt with role, skill templates, output format, and constraints.
func buildSystemMessage(skills []string) string {
	var sb systemPromptBuilder

	// Role definition
	sb.AppendLine("You are an expert task planning agent. Your job is to analyze goals and break them down into clear, actionable, and logically ordered subtasks.")

	// Skill templates
	if len(skills) > 0 {
		sb.AppendLine("")
		sb.AppendLine("## Available Skills")
		sb.AppendLine("You have access to the following planning skills. Use them to guide your analysis:")
		sb.AppendLine("")
		for _, skill := range skills {
			sb.AppendLine(fmt.Sprintf("- **%s**", skill))
		}
	}

	// Output format
	sb.AppendLine("")
	sb.AppendLine("## Output Format")
	sb.AppendLine("Return your plan as a numbered list of subtasks. Each subtask should include:")
	sb.AppendLine("1. A clear, concise title")
	sb.AppendLine("2. A brief description of what needs to be done")
	sb.AppendLine("3. Any dependencies on other subtasks")
	sb.AppendLine("")
	sb.AppendLine("Example output:")
	sb.AppendLine("1. **Analyze Requirements**")
	sb.AppendLine("   Description: Review and document all functional and non-functional requirements")
	sb.AppendLine("   Dependencies: None")
	sb.AppendLine("")
	sb.AppendLine("2. **Design Architecture**")
	sb.AppendLine("   Description: Create system architecture diagram and component interactions")
	sb.AppendLine("   Dependencies: 1")

	// Constraints
	sb.AppendLine("")
	sb.AppendLine("## Constraints")
	sb.AppendLine("- Each subtask must be independently testable")
	sb.AppendLine("- Subtasks must be ordered by dependency")
	sb.AppendLine("- Avoid over-engineering — follow the KISS principle")
	sb.AppendLine("- Apply SOLID principles where applicable")
	sb.AppendLine("- Follow DRY — no duplicated work across subtasks")
	sb.AppendLine("- Keep subtasks small and focused (one thing per task)")

	return sb.String()
}

// buildUserMessage constructs the user prompt with goal, context, and constraints.
func buildUserMessage(goal string, context string) string {
	var sb systemPromptBuilder

	sb.AppendLine("## Goal")
	sb.AppendLine(goal)

	if context != "" {
		sb.AppendLine("")
		sb.AppendLine("## Project Context")
		sb.AppendLine(context)
	}

	sb.AppendLine("")
	sb.AppendLine("Based on the goal above (and context if provided), create a detailed, actionable plan.")

	return sb.String()
}

// systemPromptBuilder is a simple helper for building multi-line prompt strings.
type systemPromptBuilder struct {
	lines []string
}

// AppendLine adds a line to the prompt builder.
func (b *systemPromptBuilder) AppendLine(line string) {
	b.lines = append(b.lines, line)
}

// String returns the built prompt as a single string.
func (b *systemPromptBuilder) String() string {
	return joinLines(b.lines)
}

// joinLines joins lines with double newlines for readability.
func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
