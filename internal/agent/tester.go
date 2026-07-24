package agent

import (
	"context"
	"fmt"
	"strings"

	"mini-orca/internal/model"
	"mini-orca/internal/types"
)

// TesterAgent is responsible for generating and analyzing tests.
type TesterAgent struct {
	skills []types.Skill
	router *model.Router
}

// NewTesterAgent creates a new tester agent.
func NewTesterAgent(skills []types.Skill, router *model.Router) *TesterAgent {
	return &TesterAgent{
		skills: skills,
		router: router,
	}
}

func (a *TesterAgent) Name() string {
	return "tester"
}

func (a *TesterAgent) Skills() []types.Skill {
	return a.skills
}

func (a *TesterAgent) Execute(ctx context.Context, input string) (*ExecutionResult, error) {
	skillPrompt := GetSkillPrompt(a.skills)

	prompt := fmt.Sprintf(`You are the Testing Agent for Mini-Orca. Your job is to analyze code and generate comprehensive tests.

## Instructions
1. Analyze the provided code carefully
2. Identify edge cases and error conditions
3. Generate comprehensive tests covering:
   - Happy path scenarios
   - Edge cases
   - Error handling
   - Boundary conditions
4. Provide a test report with coverage analysis

%s

## Output Format

### Test Analysis
[Analysis of what needs testing]

### Generated Tests
[Provide the test code]

### Test Report
- **Test Count**: [number]
- **Coverage**: [assessment]
- **Edge Cases Covered**: [list]
- **Recommendations**: [notes]

---

Code to Test:
%s`, skillPrompt, input)

	resp, err := a.router.Chat(ctx, model.PhaseTesting, []model.Message{
		{Role: "system", Content: "You are a testing agent. Generate comprehensive tests and provide coverage analysis."},
		{Role: "user", Content: prompt},
	})

	if err != nil {
		return nil, fmt.Errorf("tester execution failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("tester returned empty response")
	}

	output := strings.TrimSpace(resp.Choices[0].Message.Content)

	return &ExecutionResult{
		Output:     output,
		Phase:      string(model.PhaseTesting),
		SkillsUsed: skillNames(typesToSkills(a.skills)),
		Success:    true,
	}, nil
}
