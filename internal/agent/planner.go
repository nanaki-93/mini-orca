package agent

import (
	"context"
	"fmt"
	"strings"

	"mini-orca/internal/model"
	"mini-orca/internal/types"
)

// PlannerAgent is responsible for creating development plans.
type PlannerAgent struct {
	skills []types.Skill
	router *model.Router
}

// NewPlannerAgent creates a new planner agent.
func NewPlannerAgent(skills []types.Skill, router *model.Router) *PlannerAgent {
	return &PlannerAgent{
		skills: skills,
		router: router,
	}
}

func (a *PlannerAgent) Name() string {
	return "planner"
}

func (a *PlannerAgent) Skills() []types.Skill {
	return a.skills
}

func (a *PlannerAgent) Execute(ctx context.Context, input string) (*ExecutionResult, error) {
	skillPrompt := GetSkillPrompt(a.skills)

	prompt := fmt.Sprintf(`You are the Planning Agent for Mini-Orca. Your job is to create a detailed development plan based on the user's requirements.

## Instructions
1. Analyze the user's requirements carefully
2. Break down the work into atomic units (functions, structs, classes)
3. Identify dependencies between atomic units
4. Create a step-by-step implementation plan

%s

## Output Format
Provide your response in the following format:

### Plan Overview
[Brief description of the overall approach]

### Atomic Units
For each atomic unit, provide:
- **ID**: A unique identifier (e.g., AU-001)
- **Name**: Brief description
- **Type**: function | struct | class | interface
- **File**: Target file path
- **Description**: What this unit does
- **Dependencies**: List of AU IDs this depends on
- **Priority**: 1 (highest) to 5 (lowest)

### Dependency Graph
[Describe the order in which atomic units should be implemented]

### Notes
[Any additional notes or considerations]

---

User Requirements:
%s`, skillPrompt, input)

	resp, err := a.router.Chat(ctx, model.PhasePlanning, []model.Message{
		{Role: "system", Content: "You are a planning agent. Create detailed, actionable plans."},
		{Role: "user", Content: prompt},
	})

	if err != nil {
		return nil, fmt.Errorf("planner execution failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("planner returned empty response")
	}

	output := strings.TrimSpace(resp.Choices[0].Message.Content)

	return &ExecutionResult{
		Output:     output,
		Phase:      string(model.PhasePlanning),
		SkillsUsed: skillNames(typesToSkills(a.skills)),
		Success:    true,
	}, nil
}
