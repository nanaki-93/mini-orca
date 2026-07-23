package agent

import (
	"context"
	"fmt"
	"strings"

	"mini-orca/internal/model"
	"mini-orca/internal/types"
)

// CoderAgent is responsible for generating code for a single atomic unit.
type CoderAgent struct {
	skills []types.Skill
	router *model.Router
}

// NewCoderAgent creates a new coder agent.
func NewCoderAgent(skills []types.Skill, router *model.Router) *CoderAgent {
	return &CoderAgent{
		skills: skills,
		router: router,
	}
}

func (a *CoderAgent) Name() string {
	return "coder"
}

func (a *CoderAgent) Skills() []types.Skill {
	return a.skills
}

func (a *CoderAgent) Execute(ctx context.Context, input string) (*ExecutionResult, error) {
	skillPrompt := GetSkillPrompt(a.skills)

	prompt := fmt.Sprintf(`You are the Coding Agent for Mini-Orca. Your job is to generate HIGH QUALITY code for EXACTLY ONE atomic unit.

## Instructions
1. Read the atomic unit specification carefully
2. Generate production-quality code
3. Follow all your skills guidelines
4. Output ONLY the code - no explanations, no markdown fences
5. The code must be complete and ready to use

%s

## Critical Rules
- Generate code for ONE atomic unit only
- Do not include code for other units
- Do not repeat existing code from the file
- Use proper error handling
- Follow SOLID principles
- Keep it clean and simple (KISS)

---

Atomic Unit Specification:
%s`, skillPrompt, input)

	resp, err := a.router.Chat(ctx, model.PhaseCoding, []model.Message{
		{Role: "system", Content: "You are a coding agent. Generate clean, production-quality code for single atomic units."},
		{Role: "user", Content: prompt},
	})

	if err != nil {
		return nil, fmt.Errorf("coder execution failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("coder returned empty response")
	}

	// Clean up the output - remove markdown fences if present
	output := resp.Choices[0].Message.Content
	output = strings.TrimSpace(output)
	output = cleanCodeOutput(output)

	return &ExecutionResult{
		Output:     output,
		Phase:      string(model.PhaseCoding),
		SkillsUsed: skillNames(a.skills),
		Success:    true,
	}, nil
}

// cleanCodeOutput removes markdown code fences from the output.
func cleanCodeOutput(s string) string {
	// Remove ```language ... ``` fences
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	
	// Remove language identifier on first line
	lines := strings.SplitN(s, "\n", 2)
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		// Check if first line looks like a language identifier
		if len(firstLine) < 20 && !strings.Contains(firstLine, " ") && !strings.Contains(firstLine, "\t") {
			if len(lines) > 1 {
				return lines[1]
			}
			return ""
		}
	}
	
	return s
}
