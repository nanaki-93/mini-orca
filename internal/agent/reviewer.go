package agent

import (
	"context"
	"fmt"
	"strings"

	"mini-orca/internal/model"
	"mini-orca/internal/types"
)

// ReviewerAgent is responsible for reviewing code quality.
type ReviewerAgent struct {
	skills []types.Skill
	router *model.Router
}

// NewReviewerAgent creates a new reviewer agent.
func NewReviewerAgent(skills []types.Skill, router *model.Router) *ReviewerAgent {
	return &ReviewerAgent{
		skills: skills,
		router: router,
	}
}

func (a *ReviewerAgent) Name() string {
	return "reviewer"
}

func (a *ReviewerAgent) Skills() []types.Skill {
	return a.skills
}

func (a *ReviewerAgent) Execute(ctx context.Context, input string) (*ExecutionResult, error) {
	skillPrompt := GetSkillPrompt(a.skills)

	prompt := fmt.Sprintf(`You are the Review Agent for Mini-Orca. Your job is to review code for quality, correctness, and adherence to best practices.

## Instructions
1. Review the code thoroughly
2. Check for:
   - Code quality and style
   - Logic correctness
   - Security vulnerabilities
   - SOLID principles adherence
   - Clean code practices
   - Business logic correctness
3. Provide specific, actionable feedback
4. Rate the code on a scale of 1-10

%s

## Output Format

### Code Review Report

**Overall Rating**: [X/10]

**Strengths**:
- [List strengths]

**Issues Found**:
- [Issue 1]: [description] [severity: critical/major/minor]
- [Issue 2]: [description] [severity: critical/major/minor]

**Recommendations**:
- [Recommendation 1]
- [Recommendation 2]

**Verdict**: APPROVE | REJECT_WITH_FEEDBACK | REQUIRES_REVIEW

---

Code to Review:
%s`, skillPrompt, input)

	resp, err := a.router.Chat(ctx, model.PhaseReview, []model.Message{
		{Role: "system", Content: "You are a code review agent. Provide thorough, actionable feedback."},
		{Role: "user", Content: prompt},
	})

	if err != nil {
		return nil, fmt.Errorf("reviewer execution failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("reviewer returned empty response")
	}

	output := strings.TrimSpace(resp.Choices[0].Message.Content)

	return &ExecutionResult{
		Output:     output,
		Phase:      string(model.PhaseReview),
		SkillsUsed: skillNames(typesToSkills(a.skills)),
		Success:    true,
	}, nil
}
